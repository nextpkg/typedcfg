SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help
GOIMPORTS := go run golang.org/x/tools/cmd/goimports@v0.43.0
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
DEADCODE := go run golang.org/x/tools/cmd/deadcode@v0.43.0
LINT ?= 0

.PHONY: help precommit deadcode fmt fmt-check imports import imports-check test vet lint check check-deps check-external-consumer test-nacos-integration
help:
	@echo 'Targets: fmt imports (alias: import) fmt-check imports-check test vet lint deadcode precommit check'
	@echo 'make precommit: fix imports/formatting, then check with lint; check includes deadcode'
	@echo 'CI targets: check-external-consumer test-nacos-integration (requires Nacos)'

fmt:
	gofmt -w .

fmt-check:
	@files="$$(gofmt -l .)" && test -z "$$files" || { printf '%s\n' "$$files"; exit 1; }

import: imports
imports:
	./scripts/with-modules.sh $(GOIMPORTS) -w .

imports-check:
	@files="$$(./scripts/with-modules.sh $(GOIMPORTS) -l .)" && test -z "$$files" || { printf '%s\n' "$$files"; exit 1; }

test:
	./scripts/with-modules.sh --each go test -race ./...

vet:
	./scripts/with-modules.sh --each go vet ./...

# deadcode reports findings with exit status 0, so reject nonempty output too.
deadcode:
	@report="$$(./scripts/with-modules.sh --packages $(DEADCODE) -test)" || exit $$?; \
	if [[ -n "$$report" ]]; then printf '%s\n' "$$report"; exit 1; fi

lint:
	./scripts/with-modules.sh --each $(GOLANGCI_LINT) run ./...

# Apply fixes first, then run the full submission gate sequentially (also with -j).
precommit:
	$(MAKE) imports
	$(MAKE) check LINT=1

check: test vet fmt-check imports-check check-deps deadcode
ifeq ($(LINT),1)
check: lint
endif

# Production core dependencies must remain standard-library-only.
check-deps:
	@test "$$(go list -deps -f '{{if .Module}}{{.Module.Path}}{{end}}' github.com/nextpkg/typedcfg | sort -u)" = github.com/nextpkg/typedcfg

check-external-consumer:
	set -eu -o pipefail; \
	repo_dir="$$PWD"; \
	consumer_dir="$$(mktemp -d)"; \
	trap 'rm -rf "$$consumer_dir"' EXIT; \
	cp adapters/nacos/example/main.go "$$consumer_dir/main.go"; \
	cd "$$consumer_dir"; \
	go mod init example.com/nacos-consumer; \
	for module in . advanced validator adapters/nacos; do \
	  name="github.com/nextpkg/typedcfg"; \
	  if [[ "$$module" != "." ]]; then name="$$name/$$module"; fi; \
	  go mod edit -replace "$$name=$$repo_dir/$$module"; \
	done; \
	go get github.com/nextpkg/typedcfg/adapters/nacos@v0.0.0; \
	go mod tidy; \
	go build ./...

test-nacos-integration:
	./scripts/with-modules.sh bash -c 'cd adapters/nacos && go test -race . -run TestServerIntegration -count=1 -timeout 90s -v'
