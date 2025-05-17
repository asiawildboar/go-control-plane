package server

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	xdsservertypes "github.com/envoyproxy/go-control-plane/pkg/types"
)

var deltaErrorResponse = &cache.RawDeltaResponse{}

type streamHandler struct {
	configWatcher *cache.ConfigWatcher
	callbacks     Callbacks

	// total stream count for counting bi-di streams
	streamCount int64
	ctx         context.Context
}

// NewServer creates a delta xDS specific server which utilizes a ConfigWatcher and delta Callbacks.
func newStreamHandler(ctx context.Context, cw *cache.ConfigWatcher, callbacks Callbacks) *streamHandler {
	s := &streamHandler{
		configWatcher: cw,
		callbacks:     callbacks,
		ctx:           ctx,
	}
	return s
}

func (s *streamHandler) processDelta(str DeltaStream, reqCh <-chan *discovery.DeltaDiscoveryRequest, defaultTypeURL string) error {
	streamID := atomic.AddInt64(&s.streamCount, 1)

	// streamNonce holds a unique nonce for req-resp pairs per xDS stream.
	var streamNonce int64

	streamdata := xdsservertypes.NewStreamData()
	s.configWatcher.AddWatchingStreamData(streamdata)

	node := &core.Node{}

	defer func() {
		if s.callbacks != nil {
			s.callbacks.OnDeltaStreamClosed(streamID, node)
		}
	}()

	// sends a response, returns the new stream nonce
	send := func(resp xdsservertypes.DeltaResponse) (string, error) {
		if resp == nil {
			return "", errors.New("missing response")
		}

		response, err := resp.GetDeltaDiscoveryResponse()
		if err != nil {
			return "", err
		}

		streamNonce++
		response.Nonce = strconv.FormatInt(streamNonce, 10)
		if s.callbacks != nil {
			s.callbacks.OnStreamDeltaResponse(streamID, resp.GetDeltaRequest(), response)
		}

		return response.GetNonce(), str.Send(response)
	}

	// process a single delta response
	process := func(resp xdsservertypes.DeltaResponse) error {
		fmt.Println("pog process delta response", resp)
		typ := resp.GetDeltaRequest().GetTypeUrl()
		if resp == deltaErrorResponse {
			return status.Errorf(codes.Unavailable, "%s watch failed", typ)
		}
		nonce, err := send(resp)
		if err != nil {
			return err
		}
		streamdata.Nonce = nonce
		perTypeSubscriptionState := streamdata.PerTypeSubscriptionState[typ]
		perTypeSubscriptionState.SetResourceVersions(resp.GetVersionMap())
		return nil
	}

	// processAll purges the deltaMuxedResponses channel
	processAll := func() error {
		for {
			select {
			// We watch the multiplexed channel for incoming responses.
			case resp, more := <-streamdata.ResponseCh:
				if !more {
					break
				}
				if err := process(resp); err != nil {
					return err
				}
			default:
				return nil
			}
		}
	}

	if s.callbacks != nil {
		if err := s.callbacks.OnDeltaStreamOpen(str.Context(), streamID, defaultTypeURL); err != nil {
			return err
		}
	}

	for {
		select {
		case <-s.ctx.Done():
			return nil
		// We watch the multiplexed channel for incoming responses.
		case resp, ok := <-streamdata.ResponseCh:
			if !ok {
				break
			}
			if err := process(resp); err != nil {
				return err
			}
		case req, ok := <-reqCh:
			if !ok {
				return nil
			}
			if req == nil {
				return status.Errorf(codes.Unavailable, "empty request")
			}
			// make sure all existing responses are processed prior to new requests to avoid deadlock
			if err := processAll(); err != nil {
				return err
			}

			if s.callbacks != nil {
				if err := s.callbacks.OnStreamDeltaRequest(streamID, req); err != nil {
					return err
				}
			}

			// The node information might only be set on the first incoming delta discovery request, so store it here so we can
			// reset it on subsequent requests that omit it.
			if req.GetNode() != nil {
				node = req.GetNode()
			} else {
				req.Node = node
			}

			// type URL is required for ADS but is implicit for any other xDS stream
			if defaultTypeURL == xdsservertypes.AnyType {
				if req.GetTypeUrl() == "" {
					return status.Errorf(codes.InvalidArgument, "type URL is required for ADS")
				}
			} else if req.GetTypeUrl() == "" {
				req.TypeUrl = defaultTypeURL
			}

			typeURL := req.GetTypeUrl()

			// cancel existing watch to (re-)request a newer version
			if streamdata.PerTypeSubscriptionState[typeURL] == nil {
				// Initialize the state of the stream.
				// Since there was no previous state, we know we're handling the first request of this type
				// so we set the initial resource versions if we have any.
				// We also set the stream as wildcard based on its legacy meaning (no resource name sent in resource_names_subscribe).
				// If the state starts with this legacy mode, adding new resources will not unsubscribe from wildcard.
				// It can still be done by explicitly unsubscribing from "*"
				wildcard := len(req.GetResourceNamesSubscribe()) == 0
				versionMap := req.GetInitialResourceVersions()
				streamdata.PerTypeSubscriptionState[typeURL] = xdsservertypes.NewResourceSubscriptionState(typeURL, wildcard, versionMap)
			}
			perTypeSubscriptionState := streamdata.PerTypeSubscriptionState[typeURL]
			perTypeSubscriptionState.SetDeltaRequest(req)

			subscribe(req.GetResourceNamesSubscribe(), perTypeSubscriptionState)
			unsubscribe(req.GetResourceNamesUnsubscribe(), perTypeSubscriptionState)

			s.configWatcher.CreateDeltaWatch(perTypeSubscriptionState, streamdata.ResponseCh)
		}
	}
}

func (s *streamHandler) DeltaStreamHandler(str DeltaStream, typeURL string) error {
	// a channel for receiving incoming delta requests
	reqCh := make(chan *discovery.DeltaDiscoveryRequest)

	// we need to concurrently handle incoming requests since we kick off processDelta as a return
	go func() {
		defer close(reqCh)
		for {
			req, err := str.Recv()
			if err != nil {
				return
			}
			select {
			case reqCh <- req:
			case <-str.Context().Done():
				return
			case <-s.ctx.Done():
				return
			}
		}
	}()
	return s.processDelta(str, reqCh, typeURL)
}

// When we subscribe, we just want to make the cache know we are subscribing to a resource.
// Even if the stream is wildcard, we keep the list of explicitly subscribed resources as the wildcard subscription can be discarded later on.
func subscribe(resources []string, state *xdsservertypes.ResourceSubscriptionState) {
	sv := state.GetSubscribedResourceNames()
	for _, resource := range resources {
		if resource == "*" {
			state.SetWildcard(true)
			continue
		}
		sv[resource] = struct{}{}
	}
}

// Unsubscriptions remove resources from the stream's subscribed resource list.
// If a client explicitly unsubscribes from a wildcard request, the stream is updated and now watches only subscribed resources.
func unsubscribe(resources []string, state *xdsservertypes.ResourceSubscriptionState) {
	sv := state.GetSubscribedResourceNames()
	for _, resource := range resources {
		if resource == "*" {
			state.SetWildcard(false)
			continue
		}
		if _, ok := sv[resource]; ok && state.IsWildcard() {
			// The XDS protocol states that:
			// * if a watch is currently wildcard
			// * a resource is explicitly unsubscribed by name
			// Then the control-plane must return in the response whether the resource is removed (if no longer present for this node)
			// or still existing. In the latter case the entire resource must be returned, same as if it had been created or updated
			// To achieve that, we mark the resource as having been returned with an empty version. While creating the response, the cache will either:
			// * detect the version change, and return the resource (as an update)
			// * detect the resource deletion, and set it as removed in the response
			state.GetResourceVersions()[resource] = ""
		}
		delete(sv, resource)
	}
}
