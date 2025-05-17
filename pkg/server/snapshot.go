package xdsserver

import (
	"errors"
	"fmt"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	xdsservertypes "github.com/envoyproxy/go-control-plane/pkg/types"
	"google.golang.org/protobuf/proto"
)

// Resources is a versioned group of resources.
type Resources struct {
	// Version information.
	Version string

	// Items in the group indexed by name.
	Items map[string]proto.Message
}

// GetResourceName returns the resource name for a valid xDS response type.
func GetResourceName(res proto.Message) string {
	switch v := res.(type) {
	case *listener.Listener:
		return v.GetName()
	case *cluster.Cluster:
		return v.GetName()
	default:
		return ""
	}
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
	out := Snapshot{}

	for typ, resource := range resources {
		out.Resources[typ] = NewResources(version, resource)
	}

	fmt.Println("pog Snapshot resources:", out.Resources)
	fmt.Println("pog Snapshot GetResources:", out.GetResources(xdsservertypes.ListenerType))
	fmt.Println("pog Snapshot GetVersion:", out.GetVersion(xdsservertypes.ListenerType))
	return &out, nil
}

// GetResources selects snapshot resources by type, returning the map of resources.
func (s *Snapshot) GetResources(typeURL xdsservertypes.Type) map[string]proto.Message {
	if s == nil {
		return nil
	}
	resources := s.Resources[typeURL].Items
	withoutTTL := make(map[string]proto.Message, len(resources))

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

	// The snapshot resources never change, so no need to ever rebuild.
	if s.VersionMap != nil {
		return nil
	}

	s.VersionMap = make(map[string]map[string]string)

	for typeURL, resources := range s.Resources {
		if _, ok := s.VersionMap[typeURL]; !ok {
			s.VersionMap[typeURL] = make(map[string]string, len(resources.Items))
		}

		for _, r := range resources.Items {
			// Hash our version in here and build the version map.
			marshaledResource, err := xdsservertypes.MarshalResource(r)
			if err != nil {
				return err
			}
			v := xdsservertypes.HashResource(marshaledResource)
			if v == "" {
				return fmt.Errorf("failed to build resource version: %w", err)
			}

			s.VersionMap[typeURL][GetResourceName(r)] = v
		}
	}

	return nil
}
