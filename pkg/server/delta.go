package xdsserver

import (
	"google.golang.org/protobuf/proto"
)

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
	}, nil
}
