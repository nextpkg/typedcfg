# typedcfg

Typed configuration snapshots for Go 1.26+: JSON, HTTP, defaults, custom validation,
atomic reads and optional polling, with no third-party runtime dependencies in
the core package.

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
    if c.Port < 1 || c.Port > 65535 { return fmt.Errorf("port must be 1..65535") }
    return nil
}
func main() {
    cm := typedcfg.MustLoad[Config]("config.json", typedcfg.WithWatch())
    defer func() { if err := cm.Close(); err != nil { fmt.Println(err) } }()
    fmt.Println(cm.Get().Port)
}
```

MustLoad panics on startup errors. Load returns errors;
LoadContext[T](ctx, path, options...) also binds updates to a context.
An empty path loads only defaults. Keep the manager alive for watching and call
Get for each operation needing the latest version.

## Updates and ownership

WithWatch polls files every second and HTTP URLs every 30 seconds;
WithPollInterval adjusts this. Polling re-reads and decodes on each tick, without
ETag or unchanged-content skipping. Filesystem events are an optional adapter.

Reload starts a fresh struct, applies default tags, decodes, runs optional
validation and the struct's Validate() error, then publishes atomically.
Failed reads or invalid values retain the old version; asynchronous failures
appear on Errors() (buffer one, excess errors may be dropped).
Consume this channel in the application's existing logging loop.
Manual and automatic reloads are serialized.

Get returns a shared **read-only** pointer without copying or allocating:
do not modify any reachable field, map, slice or pointer target.
Go does not enforce this rule. Snapshot returns a private deep copy for mutation.
Sharing one mutable copy between goroutines still requires synchronization.
Publication also copies once to detach decoder/validator references.
Old snapshots remain valid; retaining them retains their memory.

Configuration must be plain data: channels, functions, unsafe pointers, locks and
unexported fields containing non-nil mutable references are rejected.
time.Time is supported. Keep runtime caches outside configuration structs.
Unexported fields are not loaded by ordinary decoding or preserved across reloads.

Close is idempotent, cancels HTTP reads, stops subscriptions and joins updates;
it retains the last snapshot. Custom sources must honor cancellation or bound I/O.
Do not call Reload/Close synchronously from a decoder/validator running inside Reload.

## Defaults and formats

Core decoding uses encoding/json directly: json tags, JSON numeric types and
UnmarshalJSON apply. There is no weak conversion of strings to numbers/durations.
For time.Duration use numeric nanoseconds in JSON or a custom type/codec.
Explicit zero/false values override defaults; missing fields retain defaults.
See [defaults](defaults/README.md) for supported tags.

WithCreateIfMissing exclusively creates missing local files from default tags
before watching. It never overwrites files or regenerates them after deletion.
Creation requires same-filesystem hard links and an encoder for custom codecs.
An invalid generated template remains available for editing.

Validate tags are **opt-in**. The core only calls custom Validate() error and any
explicit WithValidation function; it does not silently enable a tag validator.

## Optional modules

Install only capabilities used by the application.

| Capability | Import / API |
| --- | --- |
| YAML | github.com/nextpkg/typedcfg/adapters/yaml — yaml.Option(), yaml.v3 rules and yaml tags |
| File events | github.com/nextpkg/typedcfg/adapters/filewatch — filewatch.Option(path), fsnotify |
| Validation tags | github.com/nextpkg/typedcfg/validator — validator.Option(), go-playground tags |
| Multiple sources, environment, CLI | [advanced](advanced/README.md) — advanced.NewBuilder[T]() |
| Nacos / Alibaba Cloud MSE | github.com/nextpkg/typedcfg/adapters/nacos — nacos.MustLoad[T](address, dataID) |

```go
cm := typedcfg.MustLoad[Config]("config.yaml",
    yaml.Option(), filewatch.Option("config.yaml"))
```

Pass the same path to Load and filewatch.Option. Events debounce for 50ms and
survive atomic replacement; polling remains enabled as a fallback.
Use `yaml.StrictOption()` instead of `yaml.Option()` to reject unknown struct
fields and additional YAML documents. Empty input still loads defaults; map keys
remain open-ended. Invalid updates retain the previous snapshot as usual.
Independent optional modules bring their own dependencies; core go.mod test
dependencies are not included in a production core import.

The advanced module retains the former multi-file Load, MustBuild, WithTag,
provider/parser, environment and tag-validation behavior through koanf.
Its CLI provider lives at github.com/nextpkg/typedcfg/advanced/adapters/cliv3.
Nacos retains its managed-client quick start and advanced source integration;
it still carries SDK and advanced-module dependencies.

## HTTP and extension points

Load accepts an HTTP(S) URL as its path. WithHTTP(HTTPOptions{...}) configures
headers, client, timeout and body limit; defaults are 10 seconds and 1MiB.
Only 2xx responses are accepted. Redirects must stay on the original
scheme/host/port and are limited to ten requests. URL userinfo and fragments
are rejected. Headers and the client are copied when loading; request errors
omit the full URL so signed query values are not echoed.
Select a YAML codec explicitly for YAML URLs. No disk cache, remote writes,
environment expansion or credential rotation is provided.

LoadSource[T](ctx, source, options...) accepts a caller-owned byte reader;
WithCodec supplies direct decoding/encoding, WithValidation adds validation,
and WithSubscription registers notifications plus polling fallback.
Registration precedes the initial read and is cleaned up on startup failure.
