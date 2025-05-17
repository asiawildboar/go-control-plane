package xdsserver

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"
)

// Resources is a versioned group of resources.
type Resources struct {
	// Version information.
	Version string

	// Items in the group indexed by name.
	Items map[string]proto.Message
}

// IndexResourcesByName creates a map from the resource name to the resource.
func IndexResourcesByName(items []proto.Message) map[string]proto.Message {
	indexed := make(map[string]proto.Message, len(items))
	for _, item := range items {
		indexed[GetResourceName(item)] = item
	}
	return indexed
}

// NewResources creates a new resource group.
func NewResources(version string, items []proto.Message) Resources {
	return Resources{
		Version: version,
		Items:   IndexResourcesByName(items),
	}
}

type Snapshot struct {
	Resources map[string]Resources

	VersionMap map[string]map[string]string
}

// NewSnapshot creates a snapshot from response types and a version.
// The resources map is keyed off the type URL of a resource, followed by the slice of resource objects.
func NewSnapshot(version string, resources map[string][]proto.Message) (*Snapshot, error) {
	out := Snapshot{
		Resources:  make(map[string]Resources, len(resources)),
		VersionMap: make(map[string]map[string]string),
	}

	for typ, resource := range resources {
		out.Resources[typ] = NewResources(version, resource)
	}

	// Construct the version map for the snapshot.
	if err := out.ConstructVersionMap(); err != nil {
		return nil, fmt.Errorf("failed to construct version map: %w", err)
	}

	fmt.Println("[NewSnapshot] resources:", out, "GetResources", out.GetResources(ListenerType), "GetVersion", out.GetVersion(ListenerType))
	return &out, nil
}

// GetResources selects snapshot resources by type, returning the map of resources.
func (s *Snapshot) GetResources(typeURL Type) map[string]proto.Message {
	if s == nil {
		return nil
	}
	resources := s.Resources[typeURL].Items
	res := make(map[string]proto.Message, len(resources))

	for k, v := range resources {
		res[k] = v
	}
	return res
}

// GetVersion returns the version for a resource type.
func (s *Snapshot) GetVersion(typeURL Type) string {
	if s == nil {
		return ""
	}
	return s.Resources[typeURL].Version
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

	s.VersionMap = make(map[string]map[string]string)

	for typeURL, resources := range s.Resources {
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
