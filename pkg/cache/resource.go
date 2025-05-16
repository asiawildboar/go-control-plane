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
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/resource"
	"google.golang.org/protobuf/proto"
)

// MarshalResource converts the Resource to MarshaledResource.
func MarshalResource(resource Resource) (MarshaledResource, error) {
	return proto.MarshalOptions{Deterministic: true}.Marshal(resource)
}

// HashResource will take a resource and create a SHA256 hash sum out of the marshaled bytes
func HashResource(resource []byte) string {
	hasher := sha256.New()
	hasher.Write(resource)

	return hex.EncodeToString(hasher.Sum(nil))
}

// GetResourceName returns the resource names for a list of valid xDS response types.
func GetResourceNames(resources []Resource) []string {
	out := make([]string, len(resources))
	for i, r := range resources {
		out[i] = GetResourceName(r)
	}
	return out
}

func GetResponseType(typeURL resource.Type) types.ResponseType {
	switch typeURL {
	case resource.EndpointType:
		return types.Endpoint
	case resource.ClusterType:
		return types.Cluster
	case resource.RouteType:
		return types.Route
	case resource.ScopedRouteType:
		return types.ScopedRoute
	case resource.VirtualHostType:
		return types.VirtualHost
	case resource.ListenerType:
		return types.Listener
	case resource.SecretType:
		return types.Secret
	case resource.RuntimeType:
		return types.Runtime
	case resource.ExtensionConfigType:
		return types.ExtensionConfig
	case resource.RateLimitConfigType:
		return types.RateLimitConfig
	}
	return types.UnknownType
}

// GetResponseTypeURL returns the type url for a valid enum.
func GetResponseTypeURL(responseType types.ResponseType) (string, error) {
	switch responseType {
	case types.Endpoint:
		return resource.EndpointType, nil
	case types.Cluster:
		return resource.ClusterType, nil
	case types.Route:
		return resource.RouteType, nil
	case types.ScopedRoute:
		return resource.ScopedRouteType, nil
	case types.VirtualHost:
		return resource.VirtualHostType, nil
	case types.Listener:
		return resource.ListenerType, nil
	case types.Secret:
		return resource.SecretType, nil
	case types.Runtime:
		return resource.RuntimeType, nil
	case types.ExtensionConfig:
		return resource.ExtensionConfigType, nil
	case types.RateLimitConfig:
		return resource.RateLimitConfigType, nil
	default:
		return "", fmt.Errorf("couldn't map response type %v to known resource type", responseType)
	}
}
