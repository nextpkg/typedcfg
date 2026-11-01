# Default values

`typedcfg` applies defaults before decoding sources; decoded explicit zero values are preserved. The standalone helper `defaults.SetDefaults(&cfg)` fills zero fields on an existing struct.

```go
type Config struct {
    Port int `json:"port" default:"8080"`
    Timeout time.Duration `json:"timeout" default:"30s"`
    Tags []string `json:"tags" default:"web,api"`
    Database struct {
        Host string `json:"host" default:"localhost"`
    } `json:"database"`
}
```

Supported tags: strings, signed/unsigned integers, floats, booleans, `time.Duration`, comma-separated string slices, and pointers to these types. Nested value structs are traversed automatically. Unsupported field kinds do not receive a default.

A nil pointer needs its own `default` tag to be allocated; for a struct pointer use `default:""` so nested defaults are visited. Non-nil pointers are preserved by this helper. Defaults are not recursively applied to collection elements or objects allocated later by the decoder. For straightforward predictable defaults, prefer nested value structs.

```go
cfg := Config{}
if err := defaults.SetDefaults(&cfg); err != nil {
    return err
}
```
