#!/usr/bin/env bash
set -euo pipefail
repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"
for module in . advanced validator adapters/yaml adapters/filewatch adapters/nacos; do
  (
    cd "$module"
    go test -race ./...
    go vet ./...
    if [[ "${LINT:-0}" == 1 ]]; then
      go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
    fi
  )
done
# Production core dependencies must remain standard-library-only.
test "$(go list -deps -f '{{if .Module}}{{.Module.Path}}{{end}}' github.com/nextpkg/typedcfg | sort -u)" = github.com/nextpkg/typedcfg
test -z "$(gofmt -l .)"
