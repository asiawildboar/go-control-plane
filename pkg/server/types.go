package xdsserver

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	tls "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"

	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Type is an alias to string which we expose to users of the snapshot API which accepts `resource.Type` resource URLs.
type Type = string

// Resource types in xDS v3.
const (
	APITypePrefix       = "type.googleapis.com/"
	EndpointType        = APITypePrefix + "envoy.config.endpoint.v3.ClusterLoadAssignment"
	ClusterType         = APITypePrefix + "envoy.config.cluster.v3.Cluster"
	RouteType           = APITypePrefix + "envoy.config.route.v3.RouteConfiguration"
	ScopedRouteType     = APITypePrefix + "envoy.config.route.v3.ScopedRouteConfiguration"
	VirtualHostType     = APITypePrefix + "envoy.config.route.v3.VirtualHost"
	ListenerType        = APITypePrefix + "envoy.config.listener.v3.Listener"
	SecretType          = APITypePrefix + "envoy.extensions.transport_sockets.tls.v3.Secret"
	ExtensionConfigType = APITypePrefix + "envoy.config.core.v3.TypedExtensionConfig"
	RuntimeType         = APITypePrefix + "envoy.service.runtime.v3.Runtime"
	ThriftRouteType     = APITypePrefix + "envoy.extensions.filters.network.thrift_proxy.v3.RouteConfiguration"

	// Rate Limit service
	RateLimitConfigType = APITypePrefix + "ratelimit.config.ratelimit.v3.RateLimitConfig"

	// AnyType is used only by ADS
	AnyType = ""
)

// ResponseType enumeration of supported response types
type ResponseType int

// NOTE: The order of this enum MATTERS!
// https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol#aggregated-discovery-service
// ADS expects things to be returned in a specific order.
// See the following issue for details: https://github.com/envoyproxy/go-control-plane/issues/526
const (
	Cluster ResponseType = iota
	Endpoint
	Listener
	Route
	ScopedRoute
	VirtualHost
	Secret
	Runtime
	ExtensionConfig
	RateLimitConfig
	UnknownType // token to count the total number of supported types
)

// GetResourceName returns the resource names for a list of valid xDS response types.
func GetResourceNames(resources []proto.Message) []string {
	out := make([]string, len(resources))
	for i, r := range resources {
		out[i] = GetResourceName(r)
	}
	return out
}

// GetResourceName returns the resource name for a valid xDS response type.
func GetResourceName(res proto.Message) string {
	switch v := res.(type) {
	case *listener.Listener:
		return v.GetName()
	case *cluster.Cluster:
		return v.GetName()
	case *tls.Secret:
		return v.GetName()
	default:
		return ""
	}
}

// MarshalResource converts the Resource to MarshaledResource.
func MarshalResource(resource proto.Message) ([]byte, error) {
	return proto.MarshalOptions{Deterministic: true}.Marshal(resource)
}

// HashResource will take a resource and create a SHA256 hash sum out of the marshaled bytes
func HashResource(resource []byte) string {
	hasher := sha256.New()
	hasher.Write(resource)

	return hex.EncodeToString(hasher.Sum(nil))
}

// DeltaResponse is a wrapper around Envoy's DeltaDiscoveryResponse
type DeltaResponseWrapper struct {
	// For debugging purposes only
	DeltaRequest *discovery.DeltaDiscoveryRequest

	// Request is the latest delta request on the stream.
	TypeURL string

	// SystemVersionInfo holds the currently applied response system version and should be used for debugging purposes only.
	SystemVersionInfo string

	// Resources to be included in the response.
	Resources []proto.Message

	// RemovedResources is a list of resource aliases which should be dropped by the consuming client.
	RemovedResources []string

	// VersionMap consists of updated version mappings after this response is applied
	VersionMap map[string]string

	// nonce is the nonce to be used in the delta response.
	Nonce int64
}

// groups together resource-related arguments for the createDeltaResponse function
type resourceContainer struct {
	resourceMap   map[string]proto.Message
	versionMap    map[string]string
	systemVersion string
}

func CreateDeltaResponse(state *ResourceSubscriptionState, resources resourceContainer) (*DeltaResponseWrapper, error) {
	// variables to build our response with
	var nextVersionMap map[string]string
	var filtered []proto.Message
	var toRemove []string

	// If we are handling a wildcard request, we want to respond with all resources
	switch {
	case state.IsWildcard():
		if len(state.GetResourceVersions()) == 0 {
			filtered = make([]proto.Message, 0, len(resources.resourceMap))
		}
		nextVersionMap = make(map[string]string, len(resources.resourceMap))
		for name, r := range resources.resourceMap {
			// Since we've already precomputed the version hashes of the new snapshot,
			// we can just set it here to be used for comparison later
			version := resources.versionMap[name]
			nextVersionMap[name] = version
			prevVersion, found := state.GetResourceVersions()[name]
			if !found || (prevVersion != version) {
				filtered = append(filtered, r)
			}
		}

		// Compute resources for removal
		// The resource version can be set to "" here to trigger a removal even if never returned before
		for name := range state.GetResourceVersions() {
			if _, ok := resources.resourceMap[name]; !ok {
				toRemove = append(toRemove, name)
			}
		}
	default:
		nextVersionMap = make(map[string]string, len(state.GetSubscribedResourceNames()))
		// state.GetResourceVersions() may include resources no longer subscribed
		// In the current code this gets silently cleaned when updating the version map
		for name := range state.GetSubscribedResourceNames() {
			prevVersion, found := state.GetResourceVersions()[name]
			if r, ok := resources.resourceMap[name]; ok {
				nextVersion := resources.versionMap[name]
				if prevVersion != nextVersion {
					filtered = append(filtered, r)
				}
				nextVersionMap[name] = nextVersion
			} else if found {
				toRemove = append(toRemove, name)
			}
		}
	}

	return &DeltaResponseWrapper{
		DeltaRequest:      state.GetDeltaRequest(),
		TypeURL:           state.GetTypeURL(),
		Resources:         filtered,
		RemovedResources:  toRemove,
		VersionMap:        nextVersionMap,
		SystemVersionInfo: resources.systemVersion,
		Nonce:             state.GetNonce() + 1,
	}, nil
}

func (d *DeltaResponseWrapper) GetDeltaDiscoveryResponse() (*discovery.DeltaDiscoveryResponse, error) {
	marshaledResources := make([]*discovery.Resource, len(d.Resources))

	for i, resource := range d.Resources {
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
				TypeUrl: d.TypeURL,
				Value:   marshaledResource,
			},
			Version: version,
		}
	}

	return &discovery.DeltaDiscoveryResponse{
		Resources:         marshaledResources,
		RemovedResources:  d.RemovedResources,
		TypeUrl:           d.TypeURL,
		SystemVersionInfo: d.SystemVersionInfo,
		Nonce:             strconv.FormatInt(d.Nonce, 10),
	}, nil
}

type ResourceSubscriptionState struct {
	// For logging and callback purposes
	deltaRequest *discovery.DeltaDiscoveryRequest

	typeURL string

	// Indicates whether the delta stream currently has a wildcard watch
	wildcard bool

	// Provides the list of resources explicitly requested by the client
	// This list might be non-empty even when set as wildcard
	subscribedResourceNames map[string]struct{}

	// ResourceVersions contains a hash of the resource as the value and the resource name as the key.
	// This field stores the last state sent to the client.
	resourceVersions map[string]string

	// First indicates whether the StreamState has been modified since its creation
	first bool

	// The nonce is used to identify the response to a specific request
	nonce int64
}

func NewResourceSubscriptionState(typeURL string, wildcard bool, initialResourceVersions map[string]string) *ResourceSubscriptionState {
	state := ResourceSubscriptionState{
		wildcard:                wildcard,
		typeURL:                 typeURL,
		subscribedResourceNames: map[string]struct{}{},
		resourceVersions:        initialResourceVersions,
		first:                   true,
		nonce:                   0,
	}
	if initialResourceVersions == nil {
		state.resourceVersions = make(map[string]string)
	}
	return &state
}

func (s *ResourceSubscriptionState) SetNonce(nonce int64) {
	s.nonce = nonce
}

func (s *ResourceSubscriptionState) GetNonce() int64 {
	return s.nonce
}

func (s *ResourceSubscriptionState) GetSubscribedResourceNames() map[string]struct{} {
	return s.subscribedResourceNames
}

func (s *ResourceSubscriptionState) SetSubscribedResourceNames(subscribedResourceNames map[string]struct{}) {
	s.subscribedResourceNames = subscribedResourceNames
}

func (s *ResourceSubscriptionState) SetWildcard(wildcard bool) {
	s.wildcard = wildcard
}

func (s *ResourceSubscriptionState) GetDeltaRequest() *discovery.DeltaDiscoveryRequest {
	return s.deltaRequest
}

func (s *ResourceSubscriptionState) SetDeltaRequest(deltaRequest *discovery.DeltaDiscoveryRequest) {
	s.deltaRequest = deltaRequest
}

// GetResourceVersions returns a map of current resources grouped by type URL.
func (s *ResourceSubscriptionState) GetResourceVersions() map[string]string {
	return s.resourceVersions
}

func (s *ResourceSubscriptionState) SetResourceVersions(resourceVersions map[string]string) {
	s.first = false
	s.resourceVersions = resourceVersions
}

func (s *ResourceSubscriptionState) IsFirst() bool {
	return s.first
}

// IsWildcard returns whether or not an xDS client requested in wildcard mode on the initial request.
func (s *ResourceSubscriptionState) IsWildcard() bool {
	return s.wildcard
}

func (s *ResourceSubscriptionState) GetTypeURL() string {
	return s.typeURL
}

type StreamData struct {
	// Opaque resources share a muxed channel
	ResponseCh chan *DeltaResponseWrapper
	Nonce      string

	PerTypeSubscriptionState map[string]*ResourceSubscriptionState
}

func NewStreamData() *StreamData {
	// responseCh needs a buffer to release go-routines populating it
	//
	// because responseCh can be populated by an update from the cache
	// and a request from the client, we need to create the channel with
	// a buffersize of 2x the number of types to avoid deadlocks.
	return &StreamData{
		ResponseCh:               make(chan *DeltaResponseWrapper, 10),
		PerTypeSubscriptionState: make(map[string]*ResourceSubscriptionState),
	}
}
