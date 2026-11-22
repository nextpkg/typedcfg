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

Start with `typedcfg.go` for loader entry points and options, then follow
`ConfigManager.Reload` in `manager.go` from source read to atomic publication.
`Get` shares the published snapshot. `Snapshot` uses `internal/snapshot` to
return a mutable copy. The advanced loader uses the same copier.

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

Keep optional integrations in their own Go modules. Use a `Source` for
reading or decrypting bytes and a codec for decoding those bytes into the
configuration. This lets different sources share the same format checks.

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

## Features and documentation

Deliver requested features through the public loader entry points. For shared
loader options, check initial load and reload with the relevant codecs. When
a failed reload should preserve a snapshot, assert what callers see via `Get`.

Keep the README compact. Show a small configuration, the Go call and its
observable result. Update the loader options table when a public `With...`
option changes. Use core code comments to explain ownership or parsing
decisions that a reader cannot infer from the code.

## Performance work

For a library-wide review, follow the read, copy and reload paths through the
core and optional modules. Compare the baseline and candidate on the same
fixtures and toolchain. Measure the public operation as well as the hotspot,
including allocations and time per operation. Preserve isolation when reducing
reflection or copies. In a regression test, exercise the ownership boundary
directly so another protective copy cannot mask a failure.

Keep representative figures and a reproduction command in the README. Put
fixture details, raw measurements and coverage limits in the PR or report.

## Version tags

Publish requested Go module versions with Git tags. For the root module,
tag the intended commit as `vX.Y.Z` and verify the remote tag target. Create
a GitHub Release only when explicitly requested.
