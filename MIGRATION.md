# Migrating from vcfg

This is a breaking API/module migration from `github.com/nextpkg/vcfg` to `github.com/nextpkg/typedcfg`. The package name is `typedcfg`; there is no compatibility layer for the old application/plugin container.

## Module and imports

After the new code is merged, select a revision containing this module path:

```sh
go get github.com/nextpkg/typedcfg@main
```

Replace imports of `github.com/nextpkg/vcfg` (and the retained `defaults`/`validator` subpackages) with `github.com/nextpkg/typedcfg`, rename package qualifiers, and run `go mod tidy` after adapting the APIs below. Do not use the historical `v0.0.1` tag for the new path: that revision still declares `github.com/nextpkg/vcfg`. No new release is implied by the repository rename.

Existing clones can update the remote:

```sh
git remote set-url origin https://github.com/nextpkg/typedcfg.git
```

## API changes

| Before | Now |
| --- | --- |
| `MustLoad[T](sources ...any)` | `Load[T](files ...string)` returns manager/error; `MustLoad` accepts files and panics on errors |
| `AddProvider(provider)` with guessed parser | `AddProvider(provider, parser)`; use nil for map providers |
| `AddCliFlags(cmd, ".")` | `AddProvider(cliv3.Provider(cmd, "server.port", "enabled"), nil)` from `adapters/cliv3` |
| Implicit `koanf` decoding, examples often used JSON tags | `json` tags are the default for every format; select `WithTag("koanf")` for existing koanf structs |
| `APP_SERVER_PORT` | `APP_SERVER__PORT`; single underscores are literal, e.g. `APP_DATABASE__MAX_CONNS` |
| `EnableWatch` / `DisableWatch` | Choose `WithWatch()` when building; cancel the Build context or call `Close` to stop; build a new manager to restart |
| `CloseWithContext(ctx)` | `Close()`; the Build context controls the watcher lifetime |
| `WithPlugin`, `EnablePlugins`, `StartPlugins`, `StopPlugins`, `MustEnableAndStartPlugins` | Removed; initialize and close application components explicitly |
| `plugins`, `plugins/builtins`, `slogs`, `providers` packages | Removed; use explicit koanf provider/parser pairs and application-owned logging |
| `ErrorTypePluginFailure` | Removed with the plugin subsystem |

`MustBuild[T](files...)` and `Builder.MustBuild()` remain available. Building with no sources now loads defaults and validates them; watching without an AddFile source returns an error. Unsupported file extensions return errors rather than falling back to YAML.

## Application components

Remove embedded `plugins.BaseConfig` and model settings as ordinary structs. Read a snapshot, then construct the logger/database/cache in application code. Use the standard `log/slog` package or the application's existing logging library; typedcfg does not register a global logger or perform file rotation.

Configuration reload does not automatically rebuild application services. Applications that need coordinated service replacement must validate/build the replacement, switch their own references, and close old resources themselves. Reading a new configuration snapshot is not proof that a database connection or logger has been reconfigured.

## Reload and ownership

Reload now discards stale merged state; a removed key returns to its default or zero value. Invalid new state leaves the previous snapshot intact. Treat `Get()` and all reachable objects as immutable; copy into application-owned data before modifying them.

Consume `Errors()` when watching to observe failures. It is bounded to one pending error, not an audit log. Custom providers are re-read by `Reload`, not watched or closed automatically, and must provide their own timeouts. Keep their backing state synchronized if another goroutine changes it.

## Rollback

To roll back an application migration, revert its import/API/environment changes and restore its previous go.mod/go.sum (including the old vcfg revision); the historical revision is retained in Git. A repository rename does not rewrite historical module declarations. Repository-level rollback requires the owner to rename it back only after coordinating consumers of the new URL; avoid reusing either URL for an unrelated repository during migration.
