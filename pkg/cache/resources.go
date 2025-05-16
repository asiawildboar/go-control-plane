package cache

import (
	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
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
