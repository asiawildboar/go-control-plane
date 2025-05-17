package xdsservertypes

import (
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
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
	GetVersionMap() map[string]string
}

type ResourceSubscriptionState struct {
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

	// For logging and callback purposes
	deltaRequest *discovery.DeltaDiscoveryRequest
}

func NewResourceSubscriptionState(typeURL string, wildcard bool, initialResourceVersions map[string]string) *ResourceSubscriptionState {
	state := ResourceSubscriptionState{
		wildcard:                wildcard,
		typeURL:                 typeURL,
		subscribedResourceNames: map[string]struct{}{},
		resourceVersions:        initialResourceVersions,
		first:                   true,
	}
	if initialResourceVersions == nil {
		state.resourceVersions = make(map[string]string)
	}
	return &state
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
	ResponseCh chan DeltaResponse
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
		ResponseCh:               make(chan DeltaResponse, 10),
		PerTypeSubscriptionState: make(map[string]*ResourceSubscriptionState),
	}
}
