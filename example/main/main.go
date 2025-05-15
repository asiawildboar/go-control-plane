// Copyright 2020 Envoyproxy Authors
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/envoyproxy/go-control-plane/example"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/server"
)

var (
	l      example.Logger
	port   uint
	nodeID string

	debugMsg = 1
)

func init() {
	l = example.Logger{}

	flag.BoolVar(&l.Debug, "debug", false, "Enable xDS server debug logging")

	// The port that this xDS server listens on
	flag.UintVar(&port, "port", 18000, "xDS management server port")

	// Tell Envoy to use this Node ID
	flag.StringVar(&nodeID, "nodeID", "test-id", "Node ID")
}

func setSnapshot(c cache.SnapshotCache) {
	// Create the snapshot that we'll serve to Envoy
	snapshot := example.GenerateSnapshot(debugMsg)
	l.Debugf("will serve snapshot %+v %d", snapshot, debugMsg)
	debugMsg++
	// Add the snapshot to the cache
	if err := c.SetSnapshot(context.Background(), nodeID, snapshot); err != nil {
		l.Errorf("snapshot error %q for %+v", err, snapshot)
		os.Exit(1)
	}
}

func SetSnapshotEvery1Minute(c cache.SnapshotCache) (bool, error) {
	setSnapshot(c)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	done := make(chan bool)
	for {
		select {
		case <-ticker.C:
			setSnapshot(c)
		case <-done:
		}
	}
}

func main() {
	flag.Parse()

	// Create a cache
	cache := cache.NewSnapshotCache(false, cache.IDHash{}, l)
	go SetSnapshotEvery1Minute(cache)

	// Run the xDS server
	ctx := context.Background()
	cb := &example.Callbacks{Debug: l.Debug}
	srv := server.NewXDSServer(ctx, cache, cb)
	example.RunServer(srv, port)
}
