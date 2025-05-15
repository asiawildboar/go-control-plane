// Package server provides an implementation of a streaming xDS server.
package server

import (
	"context"

	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/resource"
	"github.com/envoyproxy/go-control-plane/pkg/server/config"
	"google.golang.org/grpc"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	extensionconfigservice "github.com/envoyproxy/go-control-plane/envoy/service/extension/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	runtimeservice "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
)

// Server is a collection of handlers for streaming discovery requests.
type XDSServer interface {
	discovery.AggregatedDiscoveryServiceServer
}

// Callbacks is a collection of callbacks inserted into the server operation.
// The callbacks are invoked synchronously.
type Callbacks interface {
	// OnDeltaStreamOpen is called once an incremental xDS stream is open with a stream ID and the type URL (or "" for ADS).
	// Returning an error will end processing and close the stream. OnStreamClosed will still be called.
	OnDeltaStreamOpen(context.Context, int64, string) error
	// OnDeltaStreamClosed is called immediately prior to closing an xDS stream with a stream ID.
	OnDeltaStreamClosed(int64, *core.Node)
	// OnStreamDeltaRequest is called once a request is received on a stream.
	// Returning an error will end processing and close the stream. OnStreamClosed will still be called.
	OnStreamDeltaRequest(int64, *discovery.DeltaDiscoveryRequest) error
	// OnStreamDeltaResponse is called immediately prior to sending a response on a stream.
	OnStreamDeltaResponse(int64, *discovery.DeltaDiscoveryRequest, *discovery.DeltaDiscoveryResponse)
}

type serverImpl struct {
	delta *streamHandler
}

// Generic RPC Stream for the delta based xDS protocol.
type DeltaStream interface {
	grpc.ServerStream

	Send(*discovery.DeltaDiscoveryResponse) error
	Recv() (*discovery.DeltaDiscoveryRequest, error)
}

// NewXDSServer creates handlers from a config watcher and callbacks.
func NewXDSServer(ctx context.Context, config cache.ConfigWatcher, callbacks Callbacks, opts ...config.XDSOption) XDSServer {
	return &serverImpl{newStreamHandler(ctx, config, callbacks, opts...)}
}

func (s *serverImpl) StreamAggregatedResources(stream discovery.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	panic("support delta stream only.")
}

func (s *serverImpl) DeltaStreamHandler(stream DeltaStream, typeURL string) error {
	return s.delta.DeltaStreamHandler(stream, typeURL)
}

func (s *serverImpl) DeltaAggregatedResources(stream discovery.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return s.DeltaStreamHandler(stream, resource.AnyType)
}

func (s *serverImpl) DeltaEndpoints(stream endpointservice.EndpointDiscoveryService_DeltaEndpointsServer) error {
	return s.DeltaStreamHandler(stream, resource.EndpointType)
}

func (s *serverImpl) DeltaClusters(stream clusterservice.ClusterDiscoveryService_DeltaClustersServer) error {
	return s.DeltaStreamHandler(stream, resource.ClusterType)
}

func (s *serverImpl) DeltaRoutes(stream routeservice.RouteDiscoveryService_DeltaRoutesServer) error {
	return s.DeltaStreamHandler(stream, resource.RouteType)
}

func (s *serverImpl) DeltaScopedRoutes(stream routeservice.ScopedRoutesDiscoveryService_DeltaScopedRoutesServer) error {
	return s.DeltaStreamHandler(stream, resource.ScopedRouteType)
}

func (s *serverImpl) DeltaListeners(stream listenerservice.ListenerDiscoveryService_DeltaListenersServer) error {
	return s.DeltaStreamHandler(stream, resource.ListenerType)
}

func (s *serverImpl) DeltaSecrets(stream secretservice.SecretDiscoveryService_DeltaSecretsServer) error {
	return s.DeltaStreamHandler(stream, resource.SecretType)
}

func (s *serverImpl) DeltaRuntime(stream runtimeservice.RuntimeDiscoveryService_DeltaRuntimeServer) error {
	return s.DeltaStreamHandler(stream, resource.RuntimeType)
}

func (s *serverImpl) DeltaExtensionConfigs(stream extensionconfigservice.ExtensionConfigDiscoveryService_DeltaExtensionConfigsServer) error {
	return s.DeltaStreamHandler(stream, resource.ExtensionConfigType)
}

func (s *serverImpl) DeltaVirtualHosts(stream routeservice.VirtualHostDiscoveryService_DeltaVirtualHostsServer) error {
	return s.DeltaStreamHandler(stream, resource.VirtualHostType)
}
