# Repository guide

## Layout and behavior

- Root module: standard-library-only production core, default JSON decoding,
  snapshots, reloads, HTTP/file sources and defaults.
- Optional modules: `advanced`, `validator`, `adapters/yaml`,
  `adapters/filewatch`, `adapters/nacos`. Keep their dependencies out of core.
- YAML `Option()` stays permissive. `StrictOption()` rejects unknown struct
  fields and additional documents, preserving defaults, explicit zero/false
  values and free map keys. Rejected reloads retain the last valid snapshot.

## Development and validation

Use Go 1.26+ and GNU Make at the repository root. Run `make help` for targets.
Before committing, run `make precommit`: it fixes formatting/imports, then
runs all-module race tests, vet, lint, read-only formatting/import checks,
core dependency validation and deadcode. Review the resulting diff.

Individual targets: `make fmt`, `make imports` (`make import` alias),
`make test`, `make vet`, `make lint`, `make deadcode`, `make check`.
`LINT=1 make check` includes lint; `make check` is read-only.
Tools are version-pinned in Makefile and fetched via go run on first use.
Deadcode includes tests and analyzes all modules together; reports and tool
errors fail the target. Findings describe reachability in the current build
configuration: inspect public API and platform usage before removing code.

`scripts/with-modules.sh` shares the six-module list and creates an ephemeral
workspace, removed on exit. It can run once, per module (`--each`), or against
all module packages (`--packages`). Do not commit a persistent go.work.
Keep build/check orchestration in Makefile; use scripts for shared workspace
lifecycle rather than duplicating the module list.

## CI and integrations

`.github/workflows/test.yml` calls make for the check matrix, the external
Nacos consumer build (`make check-external-consumer`) and the real Nacos test
(`make test-nacos-integration`). The latter needs a running Nacos service;
`TYPEDCFG_NACOS_TEST_PORT` selects its HTTP port. `make precommit` does not
start that service or run the live integration target. Preserve this distinction
when reporting validation; a skipped service test is not a successful live test.

Keep changes focused and reuse existing tests. Add cases for important changed
behavior; avoid tests that only repeat implementation details. Do not weaken
checks to hide failures. No merge or release follows from running these commands.
