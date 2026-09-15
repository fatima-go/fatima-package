#!/bin/sh
set -eu
package_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
export GOWORK="$package_dir/go.work.dev"
cd "$package_dir"
go test -race -count=1 ./integration ../fatima-opm/... ../jupiter/deployment ../juno/deployment ../juno/control ../juno/service ../fatima-cmd/deployui ../fatima-cmd/controlui
go test ../jupiter/... ../juno/... ../fatima-cmd/... ../saturn/... ../gofar/...
