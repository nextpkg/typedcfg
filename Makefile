SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help
GOIMPORTS := go run golang.org/x/tools/cmd/goimports@v0.43.0
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
DEADCODE := go run golang.org/x/tools/cmd/deadcode@v0.43.0
LINT ?= 0

MODULES := . advanced validator adapters/yaml adapters/filewatch adapters/nacos
MODULE_PACKAGES := $(addsuffix /...,$(addprefix ./,$(MODULES)))

# Each invocation owns a workspace, including parallel make targets. One shell
# keeps the EXIT trap active through the command and propagates command errors.
define with_workspace
set -eu -o pipefail; \
repo="$$PWD"; \
scratch="$$(mktemp -d "$$repo/.module-check.XXXXXX")"; \
trap 'rm -rf "$$scratch"' EXIT; \
{ echo 'go 1.26.0'; echo 'use ('; \
  for module in $(MODULES); do \
    if [[ "$$module" == . ]]; then printf '"%s"\n' "$$repo"; \
    else printf '"%s/%s"\n' "$$repo" "$$module"; fi; \
  done; \
  echo ')'; } > "$$scratch/go.work"; \
export GOWORK="$$scratch/go.work"; \
$(1)
endef

define each_module
$(call with_workspace,for module in $(MODULES); do (cd "$$repo/$$module" && $(1)); done)
endef

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
	$(call with_workspace,$(GOIMPORTS) -w .)

imports-check:
	@$(call with_workspace,files="$$($(GOIMPORTS) -l .)"; if [[ -n "$$files" ]]; then printf '%s\n' "$$files"; exit 1; fi)

test:
	$(call each_module,go test -race ./...)

vet:
	$(call each_module,go vet ./...)

# deadcode reports findings with exit status 0, so reject nonempty output too.
deadcode:
	@$(call with_workspace,report="$$($(DEADCODE) -test $(MODULE_PACKAGES))"; if [[ -n "$$report" ]]; then printf '%s\n' "$$report"; exit 1; fi)

lint:
	$(call each_module,$(GOLANGCI_LINT) run ./...)

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
	$(call with_workspace,cd adapters/nacos && go test -race . -run TestServerIntegration -count=1 -timeout 90s -v)
