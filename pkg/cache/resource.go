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

	xdsservertypes "github.com/envoyproxy/go-control-plane/pkg/types"
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

func GetResponseType(typeURL xdsservertypes.Type) xdsservertypes.ResponseType {
	switch typeURL {
	case xdsservertypes.EndpointType:
		return xdsservertypes.Endpoint
	case xdsservertypes.ClusterType:
		return xdsservertypes.Cluster
	case xdsservertypes.RouteType:
		return xdsservertypes.Route
	case xdsservertypes.ScopedRouteType:
		return xdsservertypes.ScopedRoute
	case xdsservertypes.VirtualHostType:
		return xdsservertypes.VirtualHost
	case xdsservertypes.ListenerType:
		return xdsservertypes.Listener
	case xdsservertypes.SecretType:
		return xdsservertypes.Secret
	case xdsservertypes.RuntimeType:
		return xdsservertypes.Runtime
	case xdsservertypes.ExtensionConfigType:
		return xdsservertypes.ExtensionConfig
	case xdsservertypes.RateLimitConfigType:
		return xdsservertypes.RateLimitConfig
	}
	return xdsservertypes.UnknownType
}

// GetResponseTypeURL returns the type url for a valid enum.
func GetResponseTypeURL(responseType xdsservertypes.ResponseType) (string, error) {
	switch responseType {
	case xdsservertypes.Endpoint:
		return xdsservertypes.EndpointType, nil
	case xdsservertypes.Cluster:
		return xdsservertypes.ClusterType, nil
	case xdsservertypes.Route:
		return xdsservertypes.RouteType, nil
	case xdsservertypes.ScopedRoute:
		return xdsservertypes.ScopedRouteType, nil
	case xdsservertypes.VirtualHost:
		return xdsservertypes.VirtualHostType, nil
	case xdsservertypes.Listener:
		return xdsservertypes.ListenerType, nil
	case xdsservertypes.Secret:
		return xdsservertypes.SecretType, nil
	case xdsservertypes.Runtime:
		return xdsservertypes.RuntimeType, nil
	case xdsservertypes.ExtensionConfig:
		return xdsservertypes.ExtensionConfigType, nil
	case xdsservertypes.RateLimitConfig:
		return xdsservertypes.RateLimitConfigType, nil
	default:
		return "", fmt.Errorf("couldn't map response type %v to known resource type", responseType)
	}
}
