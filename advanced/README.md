# typedcfg advanced

Optional multi-source configuration module for Go 1.26.0+: load ordered sources, apply defaults, validate, and read a snapshot.

```go
package main

import (
    "fmt"

    typedcfg "github.com/nextpkg/typedcfg/advanced"
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

Here `rawbytes` and `json` are `github.com/knadh/koanf/providers/rawbytes` and `github.com/knadh/koanf/parsers/json`. Providers must bound their own I/O and own any resources they allocate; typedcfg does not close them. Providers implementing `WatchProvider` can be subscribed with `WithWatch()`.

## HTTP/HTTPS sources

```go
cm := typedcfg.NewBuilder[Config]().
    AddURL("https://example.com/config.yaml").
    AddEnv("APP_").
    MustBuild()
cfg := cm.Get()
```

`AddURL` fetches the URL with GET during Build and each `Reload()`. Later sources override it in the same way as local files. JSON/YAML parsing is selected from the URL path extension, ignoring query parameters; for endpoints without a supported extension, supply a parser explicitly.

```go
builder.AddURL("https://example.com/config", typedcfg.URLOptions{
    Parser:   json.Parser(),
    Timeout:  5 * time.Second,
    MaxBytes: 2 << 20,
    Headers:  http.Header{"Authorization": []string{"Bearer " + token}},
})
```

Here `json` is `github.com/knadh/koanf/parsers/json`, with `time` and `net/http` from the standard library. Defaults are a 10-second timeout and a 1 MiB response-body limit. `Client` accepts a custom `*http.Client` for transport/TLS settings; typedcfg copies the client and headers, but a supplied transport remains shared and caller-owned. A shorter Client timeout still applies.

Only 2xx responses are decoded. HTTP errors, oversized responses, parsing failures, and validation failures return an error (or panic through MustBuild); a failed Reload keeps the previous snapshot. Requests are canceled when the Build context is canceled or the manager is closed. Redirects are limited to ten visited requests and must retain the original scheme and host, including port. Use headers for credentials; URL userinfo and fragments are rejected.

Enable automatic HTTP updates with `AddURL(address).WithWatch().WithPollInterval(30 * time.Second)`. The interval defaults to 30 seconds and must be positive when specified. Each tick performs a full GET and reloads all sources; requests are serialized, with no overlapping polls. Failed fetches or invalid configurations preserve the last valid snapshot, report through `Errors()`, and are retried on the next tick. This is periodic polling, not server push; ETag/Last-Modified conditional requests are not used. Without `WithWatch`, URLs are fetched only at Build and explicit `Reload()`. `WithCreateIfMissing()` only creates local files; there is no disk cache or remote-file creation.

## Alibaba Cloud MSE Nacos

Install the optional module `github.com/nextpkg/typedcfg/adapters/nacos`:

```go
cm := nacos.MustLoad[Config]("127.0.0.1:8848", "app.yaml")
defer cm.Close()
cfg := cm.Get()
```

This creates the SDK client, loads and validates the configuration, and enables
automatic updates. Startup errors panic; use `nacos.Load[Config](ctx, address,
dataID, options...)` to handle errors. `cm.Close()` stops updates, closes the SDK
client, and removes its temporary cache. Canceling the Load context also triggers
cleanup; call Close to wait for it. Get, Reload, and Errors behave like the core
manager. The core module does not import the Nacos SDK.

Address is a hostname or IP with an optional port (default 8848); gRPC uses that
port plus 1000. DataId extensions `.json`, `.yaml`, `.yml` select the parser; a
DataId without an extension defaults to YAML. Other formats require Options.Parser.
The default namespace is public and group is DEFAULT_GROUP. Environment variables
are read once at startup:

| Variables | Purpose |
| --- | --- |
| `ALIBABA_CLOUD_ACCESS_KEY_ID`, `ALIBABA_CLOUD_ACCESS_KEY_SECRET` | MSE RAM credentials; both must be set together |
| `NACOS_USERNAME`, `NACOS_PASSWORD` | Nacos username authentication; both must be set together |
| `NACOS_NAMESPACE_ID`, `NACOS_GROUP` | Optional namespace ID and group |

For MSE, supply the instance connection hostname and credentials authorized for
the target namespace/configuration; ensure its network whitelist allows SDK
connections. Environment values are literal; `${VAR}` expansion and automatic
credential rotation are not performed.

Optional settings keep the common path short:

```go
cm := nacos.MustLoad[Config]("127.0.0.1:8848", "app.yaml", nacos.Options{
    Group: "MY_GROUP",        // Nonempty explicit values override environment defaults.
    EnvPrefix: "APP_",        // Optional business overrides, such as APP_PORT=9090.
    PollInterval: 5 * time.Second,
})
defer cm.Close()
```

The default SDK timeout is 5 seconds, with subscriptions and 30-second periodic
refresh to recover from missed notifications. Timeout, Parser, NamespaceID and
LogDir can also be set in Options. Each Load uses a fresh private cache and
disables stale snapshot fallback. SDK logs remain under the user's cache directory
at `typedcfg/nacos/log` by default (10 MiB rotation, one backup, seven-day retention).
The SDK uses a process-global logger; applications needing full SDK control should
use Provider instead. See the complete [watching example](../adapters/nacos/example/main.go):
set `NACOS_ADDRESS` and `NACOS_DATA_ID`, run it, change the remote configuration,
and observe the printed port without restarting.

For existing SDK clients or advanced connection settings, retain explicit ownership:

```go
cm := typedcfg.NewBuilder[Config]().
    AddProvider(nacos.Provider(client, "app.yaml", "DEFAULT_GROUP"), yaml.Parser()).
    WithWatch().MustBuild()
defer cm.Close() // Stop the manager before calling client.CloseClient().
```

Here `yaml` is `github.com/knadh/koanf/parsers/yaml`; use `json.Parser()` for JSON.

The adapter pins SDK v2.2.8 for static AccessKey authentication: v2.3.5 has a
data race between RAM credential refresh and request signing. Keep race testing
enabled when upgrading the SDK; applications can override module versions, so
the version selected by their dependency graph also needs verification.

With Provider, Build and Reload call the SDK's `GetConfig`. Add `.WithWatch()` to the builder to
subscribe through the SDK's `ListenConfig`; notifications automatically re-read
all sources, validate, and publish a new snapshot. No application `Reload()` call
is needed. `Close()` or Build-context cancellation removes subscriptions via
`CancelListenConfig`, but does not close the caller-owned SDK client. The adapter rejects a second subscription for the same client pointer,
DataId and Group with `nacos.ErrAlreadyWatched`, even through different Provider
objects; a failed Build releases its reservation, and successful cancellation
allows a new manager to subscribe. A failed cancellation retains the reservation
to prevent overlapping ownership. Watched clients must be pointers (as the
SDK client is). Do not also call the SDK's ListenConfig/CancelListenConfig for
that key directly, or wrap one SDK client in multiple identities: external
subscriptions are outside this adapter's ownership checks. Watch setup
errors fail Build; failed reloads report through `Errors()` and preserve the last
valid snapshot. Subscribed sources are also re-read every 30 seconds (adjust with
`WithPollInterval`) to recover from missed notifications. SDK v2.2.8 can omit
callbacks on deletion or recreation; periodic refresh reports missing content
and restores automatic updates after valid content is published again. There is
no remote creation. Empty/missing content is an error; publish `{}` explicitly
to use defaults only. Read, parse, or validation errors preserve the previous
snapshot. The SDK can fall back to local cache unless `DisableUseSnapShot` is
enabled; `NotLoadCacheAtStart` alone does not disable that fallback. Existing
SDK failover files may also override remote data. Load disables snapshot fallback and uses a fresh temporary cache directory.

When using Provider, the application owns the SDK client: configure `TimeoutMs` and call
`client.CloseClient()` after the manager is no longer used. SDK reads do not
accept a context, so canceling Build or closing the manager cannot interrupt an
in-flight SDK call; it must return before Close completes. SDK authentication,
cache, logging, and connection policies remain client responsibilities.

For the opt-in Nacos integration test and its disposable-instance requirements,
see [repository validation guidance](../AGENTS.md#ci-and-integrations).

## Create a missing configuration file

Use `WithCreateIfMissing()` to initialize missing files from the fixed `default` tags in your configuration struct:

```go
cm := typedcfg.NewBuilder[Config]().
    AddFile("config.yaml").
    WithCreateIfMissing().
    MustBuild()
cfg := cm.Get()
```

Creation is opt-in and happens once during Build, for each missing AddFile path. JSON/YAML output uses the selected decoding tag (`json` by default), includes nested defaults, and creates missing parent directories. Each missing file contains the fields serialized from a default-initialized struct using the selected tags; when layering files, those generated values participate in the normal source order. Existing files, including invalid files, are never replaced. Environment and CLI overrides are only merged in memory and never written into the generated file.

Files are written with owner-only permissions (0600), in newly created directories with 0700 permissions, subject to the platform's permission semantics. Creation requires hard-link support on the target filesystem. Concurrent initializers do not overwrite existing files or expose partially written content. Publication and temporary-file cleanup errors are returned to the caller.

Normal loading and validation follow creation; if validation fails (for example, a required setting has no default), the generated template remains for you to edit. `MustBuild` panics on creation/loading/validation errors. `Load` and `MustLoad` return an error or panic for missing files; automatic creation is available through `Builder.WithCreateIfMissing()`. Deleting a watched file after startup reports an error and preserves the last snapshot; it does not regenerate it.

## Defaults and validation

Defaults are applied before decoding, so explicit `false`, `0`, and empty strings override defaults. Missing keys retain defaults. `default` tags support primitive types, durations, string slices, and nested structs; see [defaults](../defaults/README.md) for pointer behavior.

Validation uses `validate` tags from go-playground/validator. An optional `Validate() error` method on the config pointer runs after tag validation. `T` must be a struct; unsupported configuration types return an error.

## Reload and optional watching

`WithWatch()` enables file watching, HTTP polling, and subscriptions for `WatchProvider` sources; `Load`/`MustLoad` alone do not enable watching. Keep the manager and call `cm.Get()` for each read of the current configuration: a previously saved `cfg := cm.Get()` remains the old snapshot and is not mutated in place.

`cm.Reload()` builds a fresh merged state, applies defaults, validates, and atomically publishes it. Deleted keys disappear or return to their defaults. A parse/read/validation failure keeps the previous snapshot. Both manual and automatic reloads are serialized.

```go
cm, err := typedcfg.NewBuilder[Config]().
    AddFile("config.yaml").
    WithCreateIfMissing().
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

`WithWatch` requires at least one `AddFile`, `AddURL`, or `WatchProvider` source. Watch setup failures are returned by `Build`. Parent directories are watched so atomic replacement of a target file works; unrelated filenames are ignored and target events are debounced for 50 ms. Missing/invalid files retain the previous snapshot and produce an error; restoring a valid file resumes updates. Watching starts after any requested default-file creation and before the initial source read, so subsequent file events and provider notifications are queued during that read.

`Errors()` buffers one pending asynchronous error; additional errors may be dropped until consumed. Canceling the Build context stops watching and closes the channel; `Close` also stops and joins the watcher. Without watching, the channel closes on `Close`. File failures retry on the next event or explicit `Reload`; remote sources also retry on each periodic refresh.

`Get()` returns the **shared read-only snapshot** through an atomic pointer load, without copying or allocating. Do not modify any field, map, slice, pointer target, or interface-held data reachable from it: Go does not enforce read-only pointers, and writes can corrupt configuration and race with other readers. Reload publishes a new object and does not mutate old snapshots. Fetch once per request/operation so related fields use the same version.

Use `cfg := cm.Snapshot()` when you need to modify configuration locally: it returns an independent deep copy, including nested maps, slices, pointers, arrays, and interface values. Modifying that copy does not affect the manager or other readers. Sharing the same mutable copy between goroutines still requires application synchronization.

```go
current := cm.Get()       // Shared snapshot: read only.
local := cm.Snapshot()    // Private copy: safe to edit locally.
local.Port = 9090         // Does not change current.Port or the manager.
```


Explicit Snapshot calls allocate and cost time proportional to the configuration data. Snapshot publication also copies the decoded value to detach retained provider/validator references. Configuration is data: channels, functions, unsafe pointers, synchronization primitives, and unexported fields holding non-nil mutable references cannot be safely isolated and cause Build/Reload to fail, preserving any previous snapshot. `time.Time` is supported as a value with an immutable location. Custom providers and validators must not mutate data concurrently while loading/copying is in progress. `Close` is idempotent and safe concurrently with reads/reloads; it prevents further reloads but preserves the last snapshot. A custom provider read already in progress must finish before Close returns.

## Optional CLI adapter

The optional `advanced/adapters/cliv3` package integrates urfave/cli. Inside a parsed `cli.Command.Action`, select flags explicitly:

```go
builder.AddProvider(cliv3.Provider(cmd, "server.port", "enabled"), nil)
```

Import `github.com/nextpkg/typedcfg/advanced/adapters/cliv3`. Selected flags override configuration when urfave/cli reports them as set (`cmd.IsSet`), including values from its configured environment/file sources or programmatic `Set` calls. A flag's default `Value` alone does not override configuration; explicitly set false/zero values do. Command names are never prepended. Use canonical flag names in the selection, even if the invocation uses aliases. Unselected flags such as `--config` stay outside the config map. See the executable [CLI example](example/cli/main.go).

## Examples

The [CLI example](example/cli/main.go) belongs to the advanced module.
The [basic](../example/basic/main.go) and
[watching](../example/watch/main.go) examples use the core API in the root module;
they are not advanced-module examples.

Repository development and validation instructions are in [AGENTS.md](../AGENTS.md).
