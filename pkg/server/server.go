// Package server provides an implementation of a streaming xDS server.
package server

import (
	"context"

	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/resource"
	"github.com/envoyproxy/go-control-plane/pkg/server/config"
	"github.com/envoyproxy/go-control-plane/pkg/server/delta"
	"github.com/envoyproxy/go-control-plane/pkg/server/stream"

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
type Server interface {
	discovery.AggregatedDiscoveryServiceServer

	delta.Server
}

// Callbacks is a collection of callbacks inserted into the server operation.
// The callbacks are invoked synchronously.
type Callbacks interface {
	delta.Callbacks
}

// CallbackFuncs is a convenience type for implementing the Callbacks interface.
type CallbackFuncs struct {
	DeltaStreamOpenFunc     func(context.Context, int64, string) error
	DeltaStreamClosedFunc   func(int64, *core.Node)
	StreamDeltaRequestFunc  func(int64, *discovery.DeltaDiscoveryRequest) error
	StreamDeltaResponseFunc func(int64, *discovery.DeltaDiscoveryRequest, *discovery.DeltaDiscoveryResponse)
}

var _ Callbacks = CallbackFuncs{}

// OnDeltaStreamOpen invokes DeltaStreamOpenFunc.
func (c CallbackFuncs) OnDeltaStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	if c.DeltaStreamOpenFunc != nil {
		return c.DeltaStreamOpenFunc(ctx, streamID, typeURL)
	}

	return nil
}

// OnDeltaStreamClosed invokes DeltaStreamClosedFunc.
func (c CallbackFuncs) OnDeltaStreamClosed(streamID int64, node *core.Node) {
	if c.DeltaStreamClosedFunc != nil {
		c.DeltaStreamClosedFunc(streamID, node)
	}
}

// OnStreamDeltaRequest invokes StreamDeltaResponseFunc
func (c CallbackFuncs) OnStreamDeltaRequest(streamID int64, req *discovery.DeltaDiscoveryRequest) error {
	if c.StreamDeltaRequestFunc != nil {
		return c.StreamDeltaRequestFunc(streamID, req)
	}

	return nil
}

// OnStreamDeltaResponse invokes StreamDeltaResponseFunc.
func (c CallbackFuncs) OnStreamDeltaResponse(streamID int64, req *discovery.DeltaDiscoveryRequest, resp *discovery.DeltaDiscoveryResponse) {
	if c.StreamDeltaResponseFunc != nil {
		c.StreamDeltaResponseFunc(streamID, req, resp)
	}
}

// NewServer creates handlers from a config watcher and callbacks.
func NewServer(ctx context.Context, config cache.ConfigWatcher, callbacks Callbacks, opts ...config.XDSOption) Server {
	return NewServerAdvanced(delta.NewServer(ctx, config, callbacks, opts...))
}

func NewServerAdvanced(deltaServer delta.Server) Server {
	return &server{delta: deltaServer}
}

type server struct {
	delta delta.Server
}

func (s *server) StreamAggregatedResources(stream discovery.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	panic("support delta stream only.")
}

func (s *server) DeltaStreamHandler(stream stream.DeltaStream, typeURL string) error {
	return s.delta.DeltaStreamHandler(stream, typeURL)
}

func (s *server) DeltaAggregatedResources(stream discovery.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return s.DeltaStreamHandler(stream, resource.AnyType)
}

func (s *server) DeltaEndpoints(stream endpointservice.EndpointDiscoveryService_DeltaEndpointsServer) error {
	return s.DeltaStreamHandler(stream, resource.EndpointType)
}

func (s *server) DeltaClusters(stream clusterservice.ClusterDiscoveryService_DeltaClustersServer) error {
	return s.DeltaStreamHandler(stream, resource.ClusterType)
}

func (s *server) DeltaRoutes(stream routeservice.RouteDiscoveryService_DeltaRoutesServer) error {
	return s.DeltaStreamHandler(stream, resource.RouteType)
}

func (s *server) DeltaScopedRoutes(stream routeservice.ScopedRoutesDiscoveryService_DeltaScopedRoutesServer) error {
	return s.DeltaStreamHandler(stream, resource.ScopedRouteType)
}

func (s *server) DeltaListeners(stream listenerservice.ListenerDiscoveryService_DeltaListenersServer) error {
	return s.DeltaStreamHandler(stream, resource.ListenerType)
}

func (s *server) DeltaSecrets(stream secretservice.SecretDiscoveryService_DeltaSecretsServer) error {
	return s.DeltaStreamHandler(stream, resource.SecretType)
}

func (s *server) DeltaRuntime(stream runtimeservice.RuntimeDiscoveryService_DeltaRuntimeServer) error {
	return s.DeltaStreamHandler(stream, resource.RuntimeType)
}

func (s *server) DeltaExtensionConfigs(stream extensionconfigservice.ExtensionConfigDiscoveryService_DeltaExtensionConfigsServer) error {
	return s.DeltaStreamHandler(stream, resource.ExtensionConfigType)
}

func (s *server) DeltaVirtualHosts(stream routeservice.VirtualHostDiscoveryService_DeltaVirtualHostsServer) error {
	return s.DeltaStreamHandler(stream, resource.VirtualHostType)
}
