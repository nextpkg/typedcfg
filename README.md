# typedcfg

Typed configuration snapshots for Go 1.26+: JSON, HTTP, defaults, custom validation,
atomic reads and optional polling, with no third-party runtime dependencies in
the core package.

## Load a JSON file

Save `{"port": 9000}` as `config.json`. The program prints `9000`. With `{}`
it prints the default, `8080`.

```go
package main

import (
    "fmt"

    "github.com/nextpkg/typedcfg"
)

type Config struct {
    Port int `json:"port" default:"8080"`
}

func (c *Config) Validate() error {
    if c.Port < 1 || c.Port > 65535 {
        return fmt.Errorf("port must be 1..65535")
    }
    return nil
}

func main() {
    cm := typedcfg.MustLoad[Config]("config.json")
    defer cm.Close()
    fmt.Println(cm.Get().Port)
}
```

Use `Load` to handle startup errors or `MustLoad` to panic on failure.
An empty path loads defaults only. To create a missing file from the defaults,
pass `WithCreateIfMissing()`. Existing files are preserved.

## Loader options

| Core option | Use |
| --- | --- |
| `WithCreateIfMissing()` | Create a missing local file from defaults. |
| `WithStrictJSON()` | Reject duplicate object keys and unknown struct fields. |
| `WithCodec(decode, encode)` | Supply a decoder and an optional encoder for file creation. |
| `WithEnvExpansion(enabled)` | Expand `${NAME}` in string values before validation. Off by default. |
| `WithValidation(fn)` | Run a validation function before the config’s `Validate()` method. |
| `WithWatch()` | Enable periodic reloads. |
| `WithPollInterval(d)` | Set the interval for enabled polling. |
| `WithSubscription(subscribe)` | Reload on notifications with polling as a fallback. |
| `WithHTTP(opts)` | Configure the client, headers, timeout and response size limit for HTTP sources. |

The [advanced builder](advanced/README.md) also supports `WithTag`, `WithWatch`, `WithPollInterval` and `WithCreateIfMissing`.

## Reload and read snapshots

After changing the port in the file to `9100`, reload the same manager:

```go
if err := cm.Reload(); err != nil {
    return err
}
fmt.Println(cm.Get().Port) // 9100
draft := cm.Snapshot()
draft.Port = 3000          // private copy
fmt.Println(cm.Get().Port) // still 9100
```

Each reload applies defaults, decodes and validates a fresh configuration.
Readers see the new version only after success. Treat everything reachable
from `Get()` as read-only and call it again to read the latest version.
Keep configuration structs plain data, with locks and runtime caches elsewhere.

For automatic updates, pass `WithWatch()` at load time. The default poll
interval is one second for files and 30 seconds for URLs. Change it with
`WithPollInterval` and consume watch errors from `cm.Errors()`. Close the
manager on shutdown. See the [watch example](example/watch).

## Benchmarks

On an Apple M4 with Go 1.27.4, `Get()` allocated **0 B/op**. Reloading
256 string key/value pairs from JSON took **224 allocations/op** (about
56 KiB/op), including decoding and publishing the snapshot.

```sh
go test -run '^$' -bench 'GetCostSmall|MapReload/plain' -benchmem
```

## Defaults and validation

Missing fields retain their defaults. Explicit JSON values, including zero
and false, override them. The core uses Go's `encoding/json` rules. For
`time.Duration`, supply numeric nanoseconds or use a custom codec.
See the [supported default tags](defaults/README.md).

Pass `WithStrictJSON()` to reject duplicate keys and unknown struct fields.
The loader calls the struct's `Validate() error` method, as in the example
above. Use `WithValidation` for a separate validation function or the
optional validator module for validation tags.

## Environment expansion

Save this as `config.json` and run the application with `export APP_NAME=orders`.

```json
{"logDir": "/srv/${APP_NAME}/logs"}
```

```go
type Config struct {
    LogDir string `json:"logDir"`
}

cm := typedcfg.MustLoad[Config]("config.json", typedcfg.WithEnvExpansion(true))
defer cm.Close()
fmt.Println(cm.Get().LogDir) // /srv/orders/logs

literal := typedcfg.MustLoad[Config]("config.json") // expansion is off by default
defer literal.Close()
fmt.Println(literal.Get().LogDir) // /srv/${APP_NAME}/logs
```

If `APP_NAME` becomes empty or unset, `cm.Reload()` returns an error and
`cm.Get().LogDir` still holds `/srv/orders/logs`. A malformed placeholder
such as `${}` also fails the load.

The same option works with YAML and TOML codecs. Expansion runs once before
validation, including on defaults and nested string values. Map keys stay
literal. Enable it for trusted configuration sources because the file selects
which process environment variables to read.

## Optional modules

Install only capabilities used by the application.

| Capability | Import / API |
| --- | --- |
| YAML | github.com/nextpkg/typedcfg/adapters/yaml — yaml.Option() / yaml.StrictOption(), yaml.v3 rules and yaml tags |
| File events | github.com/nextpkg/typedcfg/adapters/filewatch — filewatch.Option(path), fsnotify |
| Validation tags | github.com/nextpkg/typedcfg/validator — validator.Option(), go-playground tags |
| Multiple sources, environment, CLI | [advanced](advanced/README.md) — advanced.NewBuilder[T]() |

```go
cm := typedcfg.MustLoad[Config]("config.yaml",
    yaml.Option(), filewatch.Option("config.yaml"))
```

Use the same file path for the loader and watcher. For YAML, choose
`yaml.StrictOption()` to reject unknown struct fields and extra documents.
For layered files, environment overrides or CLI flags, see the
[advanced builder examples](advanced/README.md).

## Other configuration sources

| Source | How to load it |
| --- | --- |
| HTTP(S) | Pass the URL to `Load[Config]`. Select a YAML codec explicitly for YAML responses. |
| Embedded or directory filesystem | Call `LoadFS[Config](ctx, fsys, "config/app.json")` with an `embed.FS` or `os.DirFS`. The path is relative to the filesystem root. |
| Custom source | Pass a `func(context.Context) ([]byte, error)` to `LoadSource[Config]`. Use `WithCodec` for a custom format. |

HTTP loads accept 2xx responses and allow same-origin redirects. The default
timeout is 10 seconds and the body limit is 1 MiB. Use `WithHTTP` to configure
these limits, request headers or the client.

Each reload reads the source again. Use `LoadContext` to bind a file or HTTP
manager to a context. Custom sources should honor cancellation or bound I/O.

### Read declared scalar keys

Use `github.com/nextpkg/typedcfg/lookup` with a flat struct and a lookup
function such as `os.LookupEnv`. Each load queries the exported fields
with explicit JSON names. Strings, booleans, decimal integers, finite floats and Go
durations are supported. Missing keys are omitted. An invalid value
rejects the load and a failed reload retains the previous snapshot.

```go
type Config struct {
    Timeout time.Duration `json:"TIMEOUT"`
}
source, err := lookup.Source[Config](os.LookupEnv)
if err != nil {
    return err
}
manager, err := typedcfg.LoadSource[Config](ctx, source)
// With TIMEOUT=2s, manager.Get().Timeout is 2*time.Second on success.
```
