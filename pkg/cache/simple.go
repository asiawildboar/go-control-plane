// Copyright 2018 Envoyproxy Authors
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/log"
	xdsservertypes "github.com/envoyproxy/go-control-plane/pkg/types"
)

// ResourceSnapshot is an abstract snapshot of a collection of resources that
// can be stored in a SnapshotCache. This enables applications to use the
// SnapshotCache watch machinery with their own resource types. Most
// applications will use Snapshot.
type ResourceSnapshot interface {
	// GetVersion should return the current version of the resource indicated
	// by typeURL. The version string that is returned is opaque and should
	// only be compared for equality.
	GetVersion(typeURL string) string

	// GetResourcesAndTTL returns all resources of the type indicted by
	// typeURL, together with their TTL.
	GetResourcesAndTTL(typeURL string) map[string]types.ResourceWithTTL

	// GetResources returns all resources of the type indicted by
	// typeURL. This is identical to GetResourcesAndTTL, except that
	// the TTL is omitted.
	GetResources(typeURL string) map[string]types.Resource

	// ConstructVersionMap is a hint that a delta watch will soon make a
	// call to GetVersionMap. The snapshot should construct an internal
	// opaque version string for each collection of resource types.
	ConstructVersionMap() error

	// GetVersionMap returns a map of resource name to resource version for
	// all the resources of type indicated by typeURL.
	GetVersionMap(typeURL string) map[string]string
}

// SnapshotCache is a snapshot-based cache that maintains a single versioned
// snapshot of responses per node. SnapshotCache consistently replies with the
// latest snapshot. For the protocol to work correctly in ADS mode, EDS/RDS
// requests are responded only when all resources in the snapshot xDS response
// are named as part of the request. It is expected that the CDS response names
// all EDS clusters, and the LDS response names all RDS routes in a snapshot,
// to ensure that Envoy makes the request for all EDS clusters or RDS routes
// eventually.
//
// SnapshotCache can operate as a REST or regular xDS backend. The snapshot
// can be partial, e.g. only include RDS or EDS resources.
type SnapshotCache interface {
	ConfigWatcher

	// SetSnapshot sets a response snapshot for a node. For ADS, the snapshots
	// should have distinct versions and be internally consistent (e.g. all
	// referenced resources must be included in the snapshot).
	//
	// This method will cause the server to respond to all open watches, for which
	// the version differs from the snapshot version.
	SetSnapshot(ctx context.Context, node string, snapshot ResourceSnapshot) error

	// GetSnapshots gets the snapshot for a node.
	GetSnapshot(node string) (ResourceSnapshot, error)

	// ClearSnapshot removes all status and snapshot information associated with a node.
	ClearSnapshot(node string)

	// GetStatusInfo retrieves status information for a node ID.
	GetStatusInfo(string) StatusInfo

	// GetStatusKeys retrieves node IDs for all statuses.
	GetStatusKeys() []string
}

type snapshotCache struct {
	// deltaWatchCount is atomic counters incremented for each watch respectively.
	// They need to be the first fields in the struct to guarantee 64-bit alignment,
	// which is a requirement for atomic operations on 64-bit operands to work on
	// 32-bit machines.
	deltaWatchCount int64

	log log.Logger

	// snapshots are cached resources indexed by node IDs
	snapshots map[string]ResourceSnapshot

	// status information for all nodes indexed by node IDs
	status map[string]*statusInfo

	// hash is the hashing function for Envoy nodes
	hash NodeHash

	mu sync.RWMutex
}

// NewSnapshotCache initializes a simple cache.
//
// ADS flag forces a delay in responding to streaming requests until all
// resources are explicitly named in the request. This avoids the problem of a
// partial request over a single stream for a subset of resources which would
// require generating a fresh version for acknowledgement. ADS flag requires
// snapshot consistency. For non-ADS case (and fetch), multiple partial
// requests are sent across multiple streams and re-using the snapshot version
// is OK.
//
// Logger is optional.
func NewSnapshotCache(ads bool, hash NodeHash, logger log.Logger) SnapshotCache {
	return newSnapshotCache(ads, hash, logger)
}

func newSnapshotCache(ads bool, hash NodeHash, logger log.Logger) *snapshotCache {
	if logger == nil {
		logger = log.NewDefaultLogger()
	}

	cache := &snapshotCache{
		log:       logger,
		snapshots: make(map[string]ResourceSnapshot),
		status:    make(map[string]*statusInfo),
		hash:      hash,
	}

	return cache
}

// SetSnapshotCache updates a snapshot for a node.
func (cache *snapshotCache) SetSnapshot(ctx context.Context, node string, snapshot ResourceSnapshot) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	// update the existing entry
	cache.snapshots[node] = snapshot

	// trigger existing watches for which version changed
	if info, ok := cache.status[node]; ok {
		info.mu.Lock()
		defer info.mu.Unlock()

		// Respond to delta watches for the node.
		return cache.respondDeltaWatches(ctx, info, snapshot)
	}

	return nil
}

func (cache *snapshotCache) respondDeltaWatches(ctx context.Context, info *statusInfo, snapshot ResourceSnapshot) error {
	// We only calculate version hashes when using delta. We don't
	// want to do this when using SOTW so we can avoid unnecessary
	// computational cost if not using delta.
	if len(info.deltaWatches) == 0 {
		return nil
	}

	err := snapshot.ConstructVersionMap()
	if err != nil {
		return err
	}

	// If ADS is enabled we need to order response delta watches so we guarantee
	// sending them in the correct order. Go's default implementation
	// of maps are randomized order when ranged over.
	info.orderResponseDeltaWatches()
	for _, key := range info.orderedDeltaWatches {
		watch := info.deltaWatches[key.ID]
		res, err := cache.respondDelta(
			ctx,
			snapshot,
			watch.Request,
			watch.Response,
			watch.StreamState,
		)
		if err != nil {
			return err
		}
		// If we detect a nil response here, that means there has been no state change
		// so we don't want to respond or remove any existing resource watches
		if res != nil {
			delete(info.deltaWatches, key.ID)
		}
	}

	return nil
}

// GetSnapshot gets the snapshot for a node, and returns an error if not found.
func (cache *snapshotCache) GetSnapshot(node string) (ResourceSnapshot, error) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	snap, ok := cache.snapshots[node]
	if !ok {
		return nil, fmt.Errorf("no snapshot found for node %s", node)
	}
	return snap, nil
}

// ClearSnapshot clears snapshot and info for a node.
func (cache *snapshotCache) ClearSnapshot(node string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	delete(cache.snapshots, node)
	delete(cache.status, node)
}

// CreateDeltaWatch returns a watch for a delta xDS request which implements the Simple SnapshotCache.
func (cache *snapshotCache) CreateDeltaWatch(request *DeltaRequest, state xdsservertypes.StreamState, deltaRespCh chan DeltaResponse) func() {
	nodeID := cache.hash.ID(request.GetNode())
	t := request.GetTypeUrl()

	cache.mu.Lock()
	defer cache.mu.Unlock()

	info, ok := cache.status[nodeID]
	if !ok {
		info = newStatusInfo(request.GetNode())
		cache.status[nodeID] = info
	}

	// update last watch request time
	info.setLastDeltaWatchRequestTime(time.Now())

	// find the current cache snapshot for the provided node
	snapshot, exists := cache.snapshots[nodeID]

	// There are three different cases that leads to a delayed watch trigger:
	// - no snapshot exists for the requested nodeID
	// - a snapshot exists, but we failed to initialize its version map
	// - we attempted to issue a response, but the caller is already up to date
	delayedResponse := !exists
	if exists {
		// 如果snapshot已存在
		err := snapshot.ConstructVersionMap()
		if err != nil {
			cache.log.Errorf("failed to compute version for snapshot resources inline: %s", err)
		}
		response, err := cache.respondDelta(context.Background(), snapshot, request, deltaRespCh, state)
		if err != nil {
			cache.log.Errorf("failed to respond with delta response: %s", err)
		}
		delayedResponse = response == nil
	}

	// 1. snapshot does not exist
	// 2. snapshot exists，但是没有新的资源
	if delayedResponse {
		watchID := cache.nextDeltaWatchID()

		if exists {
			cache.log.Infof("open delta watch ID:%d for %s Resources:%v from nodeID: %q,  version %q", watchID, t, state.GetSubscribedResourceNames(), nodeID, snapshot.GetVersion(t))
		} else {
			cache.log.Infof("open delta watch ID:%d for %s Resources:%v from nodeID: %q", watchID, t, state.GetSubscribedResourceNames(), nodeID)
		}

		info.setDeltaResponseWatch(watchID, DeltaResponseWatch{Request: request, Response: deltaRespCh, StreamState: state})
		return cache.cancelDeltaWatch(nodeID, watchID)
	}

	return nil
}

// Respond to a delta watch with the provided snapshot value. If the response is nil, there has been no state change.
func (cache *snapshotCache) respondDelta(ctx context.Context, snapshot ResourceSnapshot, request *DeltaRequest, value chan DeltaResponse, state xdsservertypes.StreamState) (*RawDeltaResponse, error) {
	resp := createDeltaResponse(ctx, request, state, resourceContainer{
		resourceMap:   snapshot.GetResources(request.GetTypeUrl()),
		versionMap:    snapshot.GetVersionMap(request.GetTypeUrl()),
		systemVersion: snapshot.GetVersion(request.GetTypeUrl()),
	})

	// Only send a response if there were changes
	// We want to respond immediately for the first wildcard request in a stream, even if the response is empty
	// otherwise, envoy won't complete initialization
	if len(resp.Resources) > 0 || len(resp.RemovedResources) > 0 || (state.IsWildcard() && state.IsFirst()) {
		if cache.log != nil {
			cache.log.Debugf("node: %s, sending delta response for typeURL %s with resources: %v removed resources: %v with wildcard: %t",
				request.GetNode().GetId(), request.GetTypeUrl(), GetResourceNames(resp.Resources), resp.RemovedResources, state.IsWildcard())
		}
		select {
		case value <- resp:
			return resp, nil
		case <-ctx.Done():
			return resp, context.Canceled
		}
	}
	return nil, nil
}

func (cache *snapshotCache) nextDeltaWatchID() int64 {
	return atomic.AddInt64(&cache.deltaWatchCount, 1)
}

// cancellation function for cleaning stale delta watches
func (cache *snapshotCache) cancelDeltaWatch(nodeID string, watchID int64) func() {
	return func() {
		cache.mu.RLock()
		defer cache.mu.RUnlock()
		if info, ok := cache.status[nodeID]; ok {
			info.mu.Lock()
			delete(info.deltaWatches, watchID)
			info.mu.Unlock()
		}
	}
}

// GetStatusInfo retrieves the status info for the node.
func (cache *snapshotCache) GetStatusInfo(node string) StatusInfo {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	info, exists := cache.status[node]
	if !exists {
		cache.log.Warnf("node does not exist")
		return nil
	}

	return info
}

// GetStatusKeys retrieves all node IDs in the status map.
func (cache *snapshotCache) GetStatusKeys() []string {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	out := make([]string, 0, len(cache.status))
	for id := range cache.status {
		out = append(out, id)
	}

	return out
}
