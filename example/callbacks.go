package example

import (
	"context"
	"log"
	"sync"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	xdsserver "github.com/envoyproxy/go-control-plane/pkg/server"
)

type Callbacks struct {
	Signal         chan struct{}
	Debug          bool
	DeltaRequests  int
	DeltaResponses int
	mu             sync.Mutex
}

var _ xdsserver.Callbacks = &Callbacks{}

func (cb *Callbacks) OnDeltaStreamOpen(_ context.Context, id int64, typ string) error {
	if cb.Debug {
		log.Printf("[Callbacks]-[OnDeltaStreamOpen] delta stream %d open\n", id)
	}
	return nil
}

func (cb *Callbacks) OnDeltaStreamClosed(id int64, node *core.Node) {
	if cb.Debug {
		log.Printf("delta stream %d of node %s closed\n", id, node.Id)
	}
}

func (cb *Callbacks) OnStreamDeltaResponse(id int64, req *discovery.DeltaDiscoveryRequest, res *discovery.DeltaDiscoveryResponse) {
	var resourceNames []string
	for _, resource := range res.GetResources() {
		resourceNames = append(resourceNames, resource.GetName())
	}
	log.Println("[Callbacks]-[OnStreamDeltaResponse]", req.TypeUrl, "resourceNames", resourceNames)
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.DeltaResponses++
}

func (cb *Callbacks) OnStreamDeltaRequest(id int64, req *discovery.DeltaDiscoveryRequest) error {
	log.Println("[Callbacks]-[OnStreamDeltaRequest]", req.GetTypeUrl(), "subscribe", req.GetResourceNamesSubscribe(), "unsubscribe", req.GetResourceNamesUnsubscribe(), "InitialResourceVersions", req.GetInitialResourceVersions(), "ResponseNonce", req.GetResponseNonce())
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.DeltaRequests++
	if cb.Signal != nil {
		close(cb.Signal)
		cb.Signal = nil
	}

	return nil
}
