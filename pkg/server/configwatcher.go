package xdsserver

import (
	"context"
	"fmt"
	"sync"

	"github.com/envoyproxy/go-control-plane/pkg/log"
	xdsservertypes "github.com/envoyproxy/go-control-plane/pkg/types"
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

	streamData *xdsservertypes.StreamData
	snapshot   *Snapshot

	mu sync.RWMutex
}

func NewConfigWatcher(logger log.Logger) *ConfigWatcher {
	if logger == nil {
		logger = log.NewDefaultLogger()
	}

	cache := &ConfigWatcher{
		log: logger,
	}

	return cache
}

func (cache *ConfigWatcher) AddWatchingStreamData(streamData *xdsservertypes.StreamData) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.streamData = streamData
}

// SetSnapshotCache updates a snapshot for a node.
func (cache *ConfigWatcher) SetSnapshot(snapshot *Snapshot) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	// update the existing entry
	cache.snapshot = snapshot

	// trigger existing watches for which version changed
	return cache.notify(snapshot)
}

func (cache *ConfigWatcher) notify(snapshot *Snapshot) error {
	fmt.Println("pog configwatcher notify", snapshot)
	err := snapshot.ConstructVersionMap()
	if err != nil {
		return err
	}

	if cache.streamData == nil {
		cache.log.Debugf("no stream data found, not sending response")
		return nil
	}

	fmt.Println("pog configWatcher state", cache.streamData.PerTypeSubscriptionState)

	// If ADS is enabled we need to order response delta watches so we guarantee
	// sending them in the correct order. But we care CDS and LDS only, no need
	// to order it for now.
	for _, state := range cache.streamData.PerTypeSubscriptionState {
		_, err := cache.respondDelta(context.Background(), snapshot, state, cache.streamData.ResponseCh)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetSnapshot gets the snapshot for a node, and returns an error if not found.
func (cache *ConfigWatcher) GetSnapshot() (*Snapshot, error) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	if cache.snapshot == nil {
		return nil, fmt.Errorf("no snapshot found")
	}
	return cache.snapshot, nil
}

// CreateDeltaWatch returns a watch for a delta xDS request which implements the Simple SnapshotCache.
func (cache *ConfigWatcher) CreateDeltaWatch(state *xdsservertypes.ResourceSubscriptionState, deltaRespCh chan *xdsservertypes.DeltaResponseWrapper) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	fmt.Println("pog CreateDeltaWatch", state.GetDeltaRequest().TypeUrl)

	if cache.snapshot != nil {
		// 如果snapshot已存在
		err := cache.snapshot.ConstructVersionMap()
		if err != nil {
			cache.log.Errorf("failed to compute version for snapshot resources inline: %s", err)
			return err
		}
		fmt.Println("pog CreateDeltaWatch before")
		_, err = cache.respondDelta(context.Background(), cache.snapshot, state, deltaRespCh)
		fmt.Println("pog CreateDeltaWatch after")
		if err != nil {
			cache.log.Errorf("failed to respond with delta response: %s", err)
			return err
		}
	}

	fmt.Println("pog CreateDeltaWatch returned")
	return nil
}

// Respond to a delta watch with the provided snapshot value. If the response is nil, there has been no state change.
func (cache *ConfigWatcher) respondDelta(ctx context.Context, snapshot *Snapshot, state *xdsservertypes.ResourceSubscriptionState, responseCh chan *xdsservertypes.DeltaResponseWrapper) (*xdsservertypes.DeltaResponseWrapper, error) {
	resp, err := CreateDeltaResponse(
		state,
		resourceContainer{
			resourceMap:   snapshot.GetResources(state.GetTypeURL()),
			versionMap:    snapshot.GetVersionMap(state.GetTypeURL()),
			systemVersion: snapshot.GetVersion(state.GetTypeURL()),
		},
	)
	if err != nil {
		cache.log.Errorf("failed to create delta response: %s", err)
		return nil, err
	}

	fmt.Println("pog respondDelta, snapshot.GetResources(state.GetTypeURL())", snapshot.GetResources(state.GetTypeURL()), state.GetTypeURL())
	fmt.Println("pog respondDelta, snapshot.GetVersionMap(state.GetTypeURL())", snapshot.GetVersionMap(state.GetTypeURL()), state.GetTypeURL())
	fmt.Println("pog respondDelta, napshot.GetVersion(state.GetTypeURL())", snapshot.GetVersion(state.GetTypeURL()), state.GetTypeURL())
	fmt.Println("pog respondDelta", resp)

	// Only send a response if there were changes
	// We want to respond immediately for the first wildcard request in a stream, even if the response is empty
	// otherwise, envoy won't complete initialization
	if len(resp.Resources) > 0 || len(resp.RemovedResources) > 0 || (state.IsWildcard() && state.IsFirst()) {
		if cache.log != nil {
			cache.log.Debugf("sending delta response for typeURL %s with resources: %v removed resources: %v with wildcard: %t",
				state.GetTypeURL(), state.GetSubscribedResourceNames(), resp.RemovedResources, state.IsWildcard())
		}
		select {
		case responseCh <- resp:
			return resp, nil
		case <-ctx.Done():
			return resp, context.Canceled
		}
	}
	return nil, nil
}
