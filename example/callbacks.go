package example

import (
	"context"
	"log"
	"sync"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server"
)

type Callbacks struct {
	Signal         chan struct{}
	Debug          bool
	DeltaRequests  int
	DeltaResponses int
	mu             sync.Mutex
}

var _ server.Callbacks = &Callbacks{}

func (cb *Callbacks) OnDeltaStreamOpen(_ context.Context, id int64, typ string) error {
	if cb.Debug {
		log.Printf("delta stream %d open for %s\n", id, typ)
	}
	return nil
}

func (cb *Callbacks) OnDeltaStreamClosed(id int64, node *core.Node) {
	if cb.Debug {
		log.Printf("delta stream %d of node %s closed\n", id, node.Id)
	}
}

func (cb *Callbacks) OnStreamDeltaResponse(id int64, req *discovery.DeltaDiscoveryRequest, res *discovery.DeltaDiscoveryResponse) {
	log.Println("OnStreamDeltaResponse pog", res.String())
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.DeltaResponses++
}

func (cb *Callbacks) OnStreamDeltaRequest(id int64, req *discovery.DeltaDiscoveryRequest) error {
	log.Println("OnStreamDeltaRequest pog", req.String())
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.DeltaRequests++
	if cb.Signal != nil {
		close(cb.Signal)
		cb.Signal = nil
	}

	return nil
}
