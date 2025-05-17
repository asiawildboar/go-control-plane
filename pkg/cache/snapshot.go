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
	"errors"
	"fmt"

	xdsservertypes "github.com/envoyproxy/go-control-plane/pkg/types"
	"google.golang.org/protobuf/proto"
)

type Snapshot struct {
	Resources [xdsservertypes.UnknownType]Resources

	VersionMap map[string]map[string]string
}

// NewSnapshot creates a snapshot from response types and a version.
// The resources map is keyed off the type URL of a resource, followed by the slice of resource objects.
func NewSnapshot(version string, resources map[xdsservertypes.Type][]proto.Message) (*Snapshot, error) {
	out := Snapshot{}

	for typ, resource := range resources {
		index := GetResponseType(typ)
		if index == xdsservertypes.UnknownType {
			return nil, errors.New("unknown resource type: " + typ)
		}

		out.Resources[index] = NewResources(version, resource)
	}

	fmt.Println("pog Snapshot resources:", out.Resources)
	fmt.Println("pog Snapshot GetResources:", out.GetResources(xdsservertypes.ListenerType))
	fmt.Println("pog Snapshot GetVersion:", out.GetVersion(xdsservertypes.ListenerType))
	return &out, nil
}

// GetResources selects snapshot resources by type, returning the map of resources.
func (s *Snapshot) GetResources(typeURL xdsservertypes.Type) map[string]Resource {
	if s == nil {
		return nil
	}
	typ := GetResponseType(typeURL)
	if typ == xdsservertypes.UnknownType {
		return nil
	}
	resources := s.Resources[typ].Items
	withoutTTL := make(map[string]Resource, len(resources))

	for k, v := range resources {
		withoutTTL[k] = v
	}

	return withoutTTL
}

// GetVersion returns the version for a resource type.
func (s *Snapshot) GetVersion(typeURL xdsservertypes.Type) string {
	if s == nil {
		return ""
	}
	typ := GetResponseType(typeURL)
	if typ == xdsservertypes.UnknownType {
		return ""
	}
	return s.Resources[typ].Version
}

// GetVersionMap will return the internal version map of the currently applied snapshot.
func (s *Snapshot) GetVersionMap(typeURL string) map[string]string {
	return s.VersionMap[typeURL]
}

// ConstructVersionMap will construct a version map based on the current state of a snapshot
func (s *Snapshot) ConstructVersionMap() error {
	if s == nil {
		return errors.New("missing snapshot")
	}

	// The snapshot resources never change, so no need to ever rebuild.
	if s.VersionMap != nil {
		return nil
	}

	s.VersionMap = make(map[string]map[string]string)

	for i, resources := range s.Resources {
		typeURL, err := GetResponseTypeURL(xdsservertypes.ResponseType(i))
		if err != nil {
			return err
		}
		if _, ok := s.VersionMap[typeURL]; !ok {
			s.VersionMap[typeURL] = make(map[string]string, len(resources.Items))
		}

		for _, r := range resources.Items {
			// Hash our version in here and build the version map.
			marshaledResource, err := MarshalResource(r)
			if err != nil {
				return err
			}
			v := HashResource(marshaledResource)
			if v == "" {
				return fmt.Errorf("failed to build resource version: %w", err)
			}

			s.VersionMap[typeURL][GetResourceName(r)] = v
		}
	}

	return nil
}
