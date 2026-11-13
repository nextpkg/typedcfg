# Repository guide

## Layout and behavior

- Root module: standard-library-only production core, default JSON decoding,
  snapshots, reloads, HTTP/file sources and defaults.
- Optional modules: `advanced`, `validator`, `adapters/yaml`,
  `adapters/filewatch`. Keep their dependencies out of core.
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

Makefile owns the five-module list. Public workspace targets create a private
temporary directory, run the corresponding internal `_` target, and clean up
on exit, including command failure. Use public targets so workspace setup is
not bypassed. Keep build/check orchestration in Makefile; do not commit a
persistent go.work.

## Dependencies

Use published v1.0.0 tags for internal module requirements. The development
workspace resolves local source; also validate affected adapters with GOWORK=off
when changing module requirements. Keep the Go 1.26 minimum and core's
standard-library-only runtime dependency boundary.

## CI

`.github/workflows/test.yml` runs `make check` for the Go version matrix.
The stable-Go entry enables lint; both entries include deadcode.

Prefer testify require/assert for concise test assertions; keep it a test-only
dependency. Keep changes focused and reuse existing tests. Add cases for
important changed behavior; avoid tests that only repeat implementation details. Do not weaken
checks to hide failures. No merge or release follows from running these commands.
