package xdsservertypes

// StreamState will keep track of resource cache state per type on a stream.
type StreamState struct { // nolint:golint,revive
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

	// Ordered indicates whether we want an ordered ADS stream or not
	ordered bool
}

// NewStreamState initializes a stream state.
func NewStreamState(wildcard bool, initialResourceVersions map[string]string) StreamState {
	state := StreamState{
		wildcard:                wildcard,
		subscribedResourceNames: map[string]struct{}{},
		resourceVersions:        initialResourceVersions,
		first:                   true,
		ordered:                 false, // Ordered comes from the first request since that's when we discover if they want ADS
	}

	if initialResourceVersions == nil {
		state.resourceVersions = make(map[string]string)
	}

	return state
}

// GetSubscribedResourceNames returns the list of resources currently explicitly subscribed to
// If the request is set to wildcard it may be empty
// Currently populated only when using delta-xds
func (s *StreamState) GetSubscribedResourceNames() map[string]struct{} {
	return s.subscribedResourceNames
}

// SetSubscribedResourceNames is setting the list of resources currently explicitly subscribed to
// It is decorrelated from the wildcard state of the stream
// Currently used only when using delta-xds
func (s *StreamState) SetSubscribedResourceNames(subscribedResourceNames map[string]struct{}) {
	s.subscribedResourceNames = subscribedResourceNames
}

func (s *StreamState) SetWildcard(wildcard bool) {
	s.wildcard = wildcard
}

// GetResourceVersions returns a map of current resources grouped by type URL.
func (s *StreamState) GetResourceVersions() map[string]string {
	return s.resourceVersions
}

// SetResourceVersions sets a list of resource versions by type URL and removes the flag
// of "first" since we can safely assume another request has come through the stream.
func (s *StreamState) SetResourceVersions(resourceVersions map[string]string) {
	s.first = false
	s.resourceVersions = resourceVersions
}

// IsFirst returns whether or not the state of the stream is based upon the initial request.
func (s *StreamState) IsFirst() bool {
	return s.first
}

// IsWildcard returns whether or not an xDS client requested in wildcard mode on the initial request.
func (s *StreamState) IsWildcard() bool {
	return s.wildcard
}
