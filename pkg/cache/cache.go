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
	"errors"
	"sync/atomic"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
)

// Resource is the base interface for the xDS payload.
type Resource interface {
	proto.Message
}

// MarshaledResource is an alias for the serialized binary array.
type MarshaledResource = []byte

// RawDeltaResponse is a pre-serialized xDS response that utilizes the delta discovery request/response objects.
type RawDeltaResponse struct {
	// Request is the latest delta request on the stream.
	DeltaRequest *discovery.DeltaDiscoveryRequest

	// SystemVersionInfo holds the currently applied response system version and should be used for debugging purposes only.
	SystemVersionInfo string

	// Resources to be included in the response.
	Resources []Resource

	// RemovedResources is a list of resource aliases which should be dropped by the consuming client.
	RemovedResources []string

	// VersionMap consists of updated version mappings after this response is applied
	VersionMap map[string]string

	// Marshaled Resources to be included in the response.
	marshaledResponse atomic.Value
}

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
func (r *RawDeltaResponse) GetVersionMap() map[string]string {
	return r.VersionMap
}
