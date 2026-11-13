#!/usr/bin/env bash
set -euo pipefail
repo_dir="$PWD"
consumer_dir="$(mktemp -d)"
trap 'rm -rf "$consumer_dir"' EXIT
cp adapters/nacos/example/main.go "$consumer_dir/main.go"
cd "$consumer_dir"
go mod init example.com/nacos-consumer
for module in . advanced validator adapters/nacos; do
  name="github.com/nextpkg/typedcfg"
  if [[ "$module" != "." ]]; then name="$name/$module"; fi
  go mod edit -replace "$name=$repo_dir/$module"
done
go get github.com/nextpkg/typedcfg/adapters/nacos@v0.0.0
go mod tidy
go build ./...
