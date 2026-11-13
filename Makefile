SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help
GOIMPORTS := go run golang.org/x/tools/cmd/goimports@v0.43.0
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
LINT ?= 0

.PHONY: help fmt fmt-check imports import imports-check test vet lint check check-deps check-external-consumer test-nacos-integration
help:
	@echo 'Targets: fmt imports (alias: import) fmt-check imports-check test vet lint check'
	@echo 'make check: tests, vet, formatting, imports and core dependency checks; LINT=1 adds lint'
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

lint:
	./scripts/with-modules.sh --each $(GOLANGCI_LINT) run ./...

check: test vet fmt-check imports-check check-deps
ifeq ($(LINT),1)
check: lint
endif

# Production core dependencies must remain standard-library-only.
check-deps:
	@test "$$(go list -deps -f '{{if .Module}}{{.Module.Path}}{{end}}' github.com/nextpkg/typedcfg | sort -u)" = github.com/nextpkg/typedcfg

check-external-consumer:
	bash ./scripts/check-external-consumer.sh

test-nacos-integration:
	./scripts/with-modules.sh bash -c 'cd adapters/nacos && go test -race . -run TestServerIntegration -count=1 -timeout 90s -v'
