# typedcfg

Small, typed configuration snapshots for Go 1.26.0+: load ordered sources, apply defaults, validate, and read a snapshot.

Application services are initialized by the application. There is no plugin registry, global logger, or component lifecycle container.

```go
package main

import (
    "fmt"

    "github.com/nextpkg/typedcfg"
)

type Config struct {
    Port int `json:"port" default:"8080" validate:"min=1,max=65535"`
    MaxConns int `json:"max_conns" default:"10"`
}

func main() {
    cfg := typedcfg.MustLoad[Config]("config.yaml").Get()
    fmt.Println(cfg.Port)
}
```

`MustLoad` panics if a file cannot be read, parsing fails, or validation fails, so invalid configuration stops startup immediately. For one-time loading it neither starts a watcher nor retains open files, and no `Close` is needed. Use `Load` when the caller needs to handle errors instead of panicking; both methods accept no files to load defaults and run validation.

For multiple sources at startup, `NewBuilder[Config]().AddFile("config.yaml").AddEnv("APP_").MustBuild().Get()` provides the same panic-on-error behavior. Keep the manager and call `Close` (or cancel its Build context) when using `WithWatch`.

## Sources and priority

```go
cm, err := typedcfg.NewBuilder[Config]().
    AddFile("base.yaml").
    AddFile("local.json").
    AddEnv("APP_").
    Build(ctx)
```

Later sources override earlier sources. Supported files: `.json`, `.yaml`, `.yml`; other extensions return an error. TOML and other formats require an explicit external parser through `AddProvider`.

Use `json` field tags for **all** source formats, including YAML. To use existing `koanf` or `yaml` tags, select one tag with `WithTag("koanf")` or `WithTag("yaml")`; tags are not guessed or combined.

Environment variables use `__` for nesting and retain single underscores: `APP_DATABASE__MAX_CONNS=25` maps to `database.max_conns`. Values use koanf's type conversion, including duration strings.

Custom providers explicitly declare their parser:

```go
builder.AddProvider(rawbytes.Provider([]byte(`{"port":9090}`)), json.Parser())
// A provider returning a map from Read() requires no parser:
builder.AddProvider(mapProvider, nil)
```

Here `rawbytes` and `json` are `github.com/knadh/koanf/providers/rawbytes` and `github.com/knadh/koanf/parsers/json`. Providers must bound their own I/O and own any resources they allocate; typedcfg does not close them or watch them automatically.

## Defaults and validation

Defaults are applied before decoding, so explicit `false`, `0`, and empty strings override defaults. Missing keys retain defaults. `default` tags support primitive types, durations, string slices, and nested structs; see [defaults](defaults/README.md) for pointer behavior.

Validation uses `validate` tags from go-playground/validator. An optional `Validate() error` method on the config pointer runs after tag validation. `T` must be a struct; unsupported configuration types return an error.

## Reload and optional watching

`cm.Reload()` builds a fresh merged state, applies defaults, validates, and atomically publishes it. Deleted keys disappear or return to their defaults. A parse/read/validation failure keeps the previous snapshot. Both manual and automatic reloads are serialized.

```go
cm, err := typedcfg.NewBuilder[Config]().
    AddFile("config.yaml").
    WithWatch().
    Build(ctx)
if err != nil { return err }
defer func() {
    if err := cm.Close(); err != nil { log.Printf("close configuration: %v", err) }
}()

for err := range cm.Errors() {
    log.Printf("configuration reload failed: %v", err)
}
```

`WithWatch` requires at least one `AddFile` source. Watch setup failures are returned by `Build`. Parent directories are watched so atomic replacement of a target file works; unrelated filenames are ignored and target events are debounced for 50 ms. Missing/invalid files retain the previous snapshot and produce an error; restoring a valid file resumes updates. Watch setup begins before the initial load so changes during initialization are queued.

`Errors()` buffers one pending asynchronous error; additional errors may be dropped until consumed. Canceling the Build context stops watching and closes the channel; `Close` also stops and joins the watcher. Without watching, the channel closes on `Close`. There are no automatic retries without a new file event or an explicit `Reload`.

`Get()` returns a **read-only** shared snapshot: do not mutate it or its nested maps, slices, or pointers. Readers may retain an old snapshot after a reload. `Close` is idempotent and safe concurrently with reads/reloads; it prevents further reloads but preserves the last snapshot. A custom provider read already in progress must finish before Close returns.

## Optional CLI adapter

The core package does not import urfave/cli. Inside a parsed `cli.Command.Action`, select flags explicitly:

```go
builder.AddProvider(cliv3.Provider(cmd, "server.port", "enabled"), nil)
```

Import `github.com/nextpkg/typedcfg/adapters/cliv3`. Only set flags override configuration, including explicit false/zero; command names are never prepended. Use canonical flag names in the selection, even if the invocation uses aliases. Unselected flags such as `--config` stay outside the config map. See the executable [CLI example](example/cli/main.go).

## Examples and development

Examples share the root module and compile with the library:

```sh
go run ./example/basic
go run ./example/cli --server.port 9090 --enabled=false
go run ./example/watch example/watch/config.yaml
go test ./...
go test -race ./...
go vet ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
```

See [MIGRATION.md](MIGRATION.md) for migration from vcfg, including the repository/module rename and removed APIs.
