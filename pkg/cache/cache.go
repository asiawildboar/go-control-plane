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

// Package cache defines a configuration cache for the server.
package cache

import (
	"context"
	"errors"
	"sync/atomic"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/server/stream"

	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
)

// DeltaRequest is an alias for the delta discovery request type.
type DeltaRequest = discovery.DeltaDiscoveryRequest

// Cache is a generic config cache with a watcher.
type ConfigWatcher interface {
	// ConfigWatcher requests watches for configuration resources by a node, last
	// applied version identifier, and resource names hint. The watch should send
	// the responses when they are ready. The watch can be canceled by the
	// consumer, in effect terminating the watch for the request.
	// ConfigWatcher implementation must be thread-safe.
	CreateDeltaWatch(*DeltaRequest, stream.StreamState, chan DeltaResponse) (cancel func())
}

// DeltaResponse is a wrapper around Envoy's DeltaDiscoveryResponse
type DeltaResponse interface {
	// Get the constructed DeltaDiscoveryResponse
	GetDeltaDiscoveryResponse() (*discovery.DeltaDiscoveryResponse, error)

	// Get the request that created the watch that we're now responding to. This is provided to allow the caller to correlate the
	// response with a request. Generally this will be the latest request seen on the stream for the specific type.
	GetDeltaRequest() *discovery.DeltaDiscoveryRequest

	// Get the version in the DeltaResponse. This field is generally used for debugging purposes as noted by the Envoy documentation.
	GetSystemVersion() (string, error)

	// Get the version map of the internal cache.
	// The version map consists of updated version mappings after this response is applied
	GetNextVersionMap() map[string]string

	// Get the context provided during response creation
	GetContext() context.Context
}

// RawDeltaResponse is a pre-serialized xDS response that utilizes the delta discovery request/response objects.
type RawDeltaResponse struct {
	// Request is the latest delta request on the stream.
	DeltaRequest *discovery.DeltaDiscoveryRequest

	// SystemVersionInfo holds the currently applied response system version and should be used for debugging purposes only.
	SystemVersionInfo string

	// Resources to be included in the response.
	Resources []types.Resource

	// RemovedResources is a list of resource aliases which should be dropped by the consuming client.
	RemovedResources []string

	// NextVersionMap consists of updated version mappings after this response is applied
	NextVersionMap map[string]string

	// Context provided at the time of response creation. This allows associating additional
	// information with a generated response.
	Ctx context.Context

	// Marshaled Resources to be included in the response.
	marshaledResponse atomic.Value
}

var (
	_ DeltaResponse = &RawDeltaResponse{}
)

// DeltaPassthroughResponse is a pre constructed xDS response that need not go through marshaling transformations.
type DeltaPassthroughResponse struct {
	// Request is the latest delta request on the stream
	DeltaRequest *discovery.DeltaDiscoveryRequest

	// NextVersionMap consists of updated version mappings after this response is applied
	NextVersionMap map[string]string

	// This discovery response that needs to be sent as is, without any marshaling transformations
	DeltaDiscoveryResponse *discovery.DeltaDiscoveryResponse

	ctx context.Context
}

var (
	_ DeltaResponse = &DeltaPassthroughResponse{}
)

// GetDeltaDiscoveryResponse performs the marshaling the first time its called and uses the cached response subsequently.
// We can do this because the marshaled response does not change across the calls.
// This caching behavior is important in high throughput scenarios because grpc marshaling has a cost and it drives the cpu utilization under load.
func (r *RawDeltaResponse) GetDeltaDiscoveryResponse() (*discovery.DeltaDiscoveryResponse, error) {
	marshaledResponse := r.marshaledResponse.Load()

	if marshaledResponse == nil {
		marshaledResources := make([]*discovery.Resource, len(r.Resources))

		for i, resource := range r.Resources {
			name := GetResourceName(resource)
			marshaledResource, err := MarshalResource(resource)
			if err != nil {
				return nil, err
			}
			version := HashResource(marshaledResource)
			if version == "" {
				return nil, errors.New("failed to create a resource hash")
			}
			marshaledResources[i] = &discovery.Resource{
				Name: name,
				Resource: &anypb.Any{
					TypeUrl: r.GetDeltaRequest().GetTypeUrl(),
					Value:   marshaledResource,
				},
				Version: version,
			}
		}

		marshaledResponse = &discovery.DeltaDiscoveryResponse{
			Resources:         marshaledResources,
			RemovedResources:  r.RemovedResources,
			TypeUrl:           r.GetDeltaRequest().GetTypeUrl(),
			SystemVersionInfo: r.SystemVersionInfo,
		}
		r.marshaledResponse.Store(marshaledResponse)
	}

	return marshaledResponse.(*discovery.DeltaDiscoveryResponse), nil
}

// GetDeltaRequest returns the original DeltaRequest
func (r *RawDeltaResponse) GetDeltaRequest() *discovery.DeltaDiscoveryRequest {
	return r.DeltaRequest
}

// GetSystemVersion returns the raw SystemVersion
func (r *RawDeltaResponse) GetSystemVersion() (string, error) {
	return r.SystemVersionInfo, nil
}

// NextVersionMap returns the version map which consists of updated version mappings after this response is applied
func (r *RawDeltaResponse) GetNextVersionMap() map[string]string {
	return r.NextVersionMap
}

func (r *RawDeltaResponse) GetContext() context.Context {
	return r.Ctx
}

var deltaResourceTypeURL = "type.googleapis.com/" + string(proto.MessageName(&discovery.Resource{}))

// GetDeltaDiscoveryResponse returns the final passthrough Delta Discovery Response.
func (r *DeltaPassthroughResponse) GetDeltaDiscoveryResponse() (*discovery.DeltaDiscoveryResponse, error) {
	return r.DeltaDiscoveryResponse, nil
}

// GetDeltaRequest returns the original Delta Discovery Request
func (r *DeltaPassthroughResponse) GetDeltaRequest() *discovery.DeltaDiscoveryRequest {
	return r.DeltaRequest
}

// GetSystemVersion returns the response version.
func (r *DeltaPassthroughResponse) GetSystemVersion() (string, error) {
	deltaDiscoveryResponse, _ := r.GetDeltaDiscoveryResponse()
	if deltaDiscoveryResponse != nil {
		return deltaDiscoveryResponse.GetSystemVersionInfo(), nil
	}
	return "", errors.New("DeltaDiscoveryResponse is nil")
}

// NextVersionMap returns the version map from a DeltaPassthroughResponse
func (r *DeltaPassthroughResponse) GetNextVersionMap() map[string]string {
	return r.NextVersionMap
}

func (r *DeltaPassthroughResponse) GetContext() context.Context {
	return r.ctx
}
