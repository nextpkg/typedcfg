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
`make check` validates formatting/imports without rewriting source; lint is
included when `LINT=1`. `make precommit` always enables lint.
Tools are version-pinned in Makefile and fetched via go run on first use.
Deadcode includes tests and analyzes all modules together; reports and tool
errors fail the target. Findings describe reachability in the current build
configuration: inspect public API and platform usage before removing code.

Makefile owns the six-module list. Public workspace targets create a private
temporary directory, run the corresponding internal `_` target, and clean up
on exit, including command failure. Use public targets so workspace setup is
not bypassed. Keep build/check orchestration in Makefile; do not commit a
persistent go.work.

## CI and integrations

`.github/workflows/test.yml` calls make for the check matrix, the external
Nacos consumer build (`make check-external-consumer`) and the real Nacos test
(`make test-nacos-integration`). The check matrix enables lint on stable Go;
both matrix entries include deadcode. `make check-external-consumer` is a
separate check and is not included in `make precommit`.
The live integration target needs a running Nacos service;
`TYPEDCFG_NACOS_TEST_PORT` selects its HTTP port. `make precommit` does not
start that service or run the live integration target. Preserve this distinction
when reporting validation; a skipped service test is not a successful live test.

For a disposable instance already running on localhost, run from the repository
root: `TYPEDCFG_NACOS_TEST_PORT=18848 make test-nacos-integration`.
The gRPC port must be the HTTP port plus 1000. Without the HTTP port variable,
the test skips. For a dedicated MSE test instance, also set
`TYPEDCFG_NACOS_TEST_HOST`, optional `TYPEDCFG_NACOS_TEST_NAMESPACE`, and
`ALIBABA_CLOUD_ACCESS_KEY_ID` / `ALIBABA_CLOUD_ACCESS_KEY_SECRET`.
Credentials need read/write/delete access to group `TYPEDCFG_TEST`; the test
publishes a unique DataId, waits up to 15 seconds for each value to become
readable, and removes that DataId during cleanup.

Prefer testify require/assert for concise test assertions; keep it a test-only
dependency. Keep changes focused and reuse existing tests. Add cases for
important changed behavior; avoid tests that only repeat implementation details. Do not weaken
checks to hide failures. No merge or release follows from running these commands.
