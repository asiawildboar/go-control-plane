#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

##
## Runs Envoy and the example control plane server.  See
## `internal/example` for the go source.
##

# Start envoy: important to keep drain time short
(envoy -c sample/bootstrap-delta-ads.yaml --drain-time-s 1 -l debug)&
ENVOY_PID=$!

function cleanup() {
  kill ${ENVOY_PID}
}
trap cleanup EXIT

# Run the control plane
bin/example -debug $@
