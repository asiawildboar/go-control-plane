package xdsserver

import (
	"context"

	"google.golang.org/grpc"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
)

// Server is a collection of handlers for streaming discovery requests.
type XDSServer interface {
	discovery.AggregatedDiscoveryServiceServer
	secretservice.SecretDiscoveryServiceServer
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
func NewXDSServer(ctx context.Context, config *ConfigWatcher, callbacks Callbacks) XDSServer {
	return &serverImpl{newStreamHandler(ctx, config, callbacks)}
}

func (s *serverImpl) StreamAggregatedResources(stream discovery.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	panic("support delta stream only.")
}

func (s *serverImpl) DeltaStreamHandler(stream DeltaStream, typeURL string) error {
	return s.delta.DeltaStreamHandler(stream, typeURL)
}

func (s *serverImpl) DeltaAggregatedResources(stream discovery.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return s.DeltaStreamHandler(stream, AnyType)
}

func (s *serverImpl) FetchSecrets(context.Context, *discovery.DiscoveryRequest) (*discovery.DiscoveryResponse, error) {
	panic("[FetchSecrets] support delta stream only.")
}

func (s *serverImpl) StreamSecrets(stream secretservice.SecretDiscoveryService_StreamSecretsServer) error {
	panic("[StreamSecrets] support delta stream only.")
}

func (s *serverImpl) DeltaSecrets(stream secretservice.SecretDiscoveryService_DeltaSecretsServer) error {
	return s.DeltaStreamHandler(stream, SecretType)
}
