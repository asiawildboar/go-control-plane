package xdsserver

import (
	"context"
	"fmt"
	"sync"

	"github.com/envoyproxy/go-control-plane/pkg/log"
	"google.golang.org/protobuf/proto"
)

type XDSStore struct {
	Resources map[string]proto.Message

	// VersionMap holds the current hash map of all resources in the snapshot.
	// This field should remain nil until it is used, at which point should be
	// instantiated by calling ConstructVersionMap().
	// VersionMap is only to be used with delta xDS.
	VersionMap map[string]map[string]string
}

type ConfigWatcher struct {
	log log.Logger

	streamData *StreamData
	snapshot   *Snapshot

	mu sync.RWMutex
}

func NewConfigWatcher(logger log.Logger) *ConfigWatcher {
	if logger == nil {
		logger = log.NewDefaultLogger()
	}

	cw := &ConfigWatcher{
		log: logger,
	}

	return cw
}

func (cw *ConfigWatcher) SetStreamData(streamData *StreamData) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	cw.streamData = streamData
}

func (cw *ConfigWatcher) NotifySnapshot(snapshot *Snapshot) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	// update the existing entry
	cw.snapshot = snapshot
	fmt.Println("[ConfigWatcher]-[notify]", "snapshot", snapshot)

	if cw.streamData == nil {
		cw.log.Debugf("no stream data found, not sending response")
		return nil
	}

	fmt.Println("[ConfigWatcher]-[notify]", "stream state", cw.streamData.PerTypeSubscriptionState)

	// If ADS is enabled we need to order response delta watches so we guarantee
	// sending them in the correct order. But we care CDS and LDS only, no need
	// to order it for now.
	for _, state := range cw.streamData.PerTypeSubscriptionState {
		err := cw.respondDelta(context.Background(), snapshot, state, cw.streamData.ResponseCh)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cw *ConfigWatcher) FetchSnapshot(state *ResourceSubscriptionState, deltaRespCh chan *DeltaResponseWrapper) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	fmt.Println("[ConfigWatcher]-[Fetch]", state.GetDeltaRequest().TypeUrl)

	if cw.snapshot != nil {
		err := cw.respondDelta(context.Background(), cw.snapshot, state, deltaRespCh)
		if err != nil {
			cw.log.Errorf("failed to respond with delta response: %s", err)
			return err
		}
	}
	return nil
}

// Respond to a delta watch with the provided snapshot value. If the response is nil, there has been no state change.
func (cw *ConfigWatcher) respondDelta(ctx context.Context, snapshot *Snapshot, state *ResourceSubscriptionState, responseCh chan *DeltaResponseWrapper) error {
	resp, err := CreateDeltaResponse(
		state,
		resourceContainer{
			resourceMap:   snapshot.GetResources(state.GetTypeURL()),
			versionMap:    snapshot.GetVersionMap(state.GetTypeURL()),
			systemVersion: snapshot.GetVersion(state.GetTypeURL()),
		},
	)
	if err != nil {
		cw.log.Errorf("failed to create delta response: %s", err)
		return err
	}
	fmt.Println("[ConfigWatcher]-[respondDelta]", "typeURL", state.GetTypeURL(), "resourceMap", snapshot.GetResources(state.GetTypeURL()), "versionMap", snapshot.GetVersionMap(state.GetTypeURL()), "systemVersion", snapshot.GetVersion(state.GetTypeURL()))

	// Only send a response if there were changes
	// We want to respond immediately for the first wildcard request in a stream, even if the response is empty
	// otherwise, envoy won't complete initialization
	if len(resp.Resources) > 0 || len(resp.RemovedResources) > 0 || (state.IsWildcard() && state.IsFirst()) {
		if cw.log != nil {
			cw.log.Debugf("[ConfigWatcher]-[respondDelta] sending delta response for typeURL %s with resources: %v removed resources: %v with wildcard: %t",
				state.GetTypeURL(), state.GetSubscribedResourceNames(), resp.RemovedResources, state.IsWildcard())
		}
		select {
		case responseCh <- resp:
			return nil
		case <-ctx.Done():
			return context.Canceled
		}
	}
	return nil
}
