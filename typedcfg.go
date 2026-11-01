// Package typedcfg loads ordered configuration sources into validated, typed snapshots.
// It does not initialize application services or modify global logging state.
package typedcfg

import "context"

// Load loads JSON/YAML files in order. With no files it applies defaults and validation.
// Use NewBuilder for environment variables, custom providers, or file watching.
// Load does not start watchers or retain open files; Close is optional for a
// one-time startup load.
func Load[T any](paths ...string) (*ConfigManager[T], error) {
	b := NewBuilder[T]()
	for _, path := range paths {
		b.AddFile(path)
	}
	return b.Build(context.Background())
}

// MustLoad is Load with panic-on-error semantics for application startup.
// Read, parse, default, and validation errors all panic. For one-time use:
//
//	cfg := typedcfg.MustLoad[Config]("config.yaml").Get()
func MustLoad[T any](paths ...string) *ConfigManager[T] {
	cm, err := Load[T](paths...)
	if err != nil {
		panic(err)
	}
	return cm
}

// MustBuild is an alias for MustLoad for file-only configurations.
func MustBuild[T any](paths ...string) *ConfigManager[T] { return MustLoad[T](paths...) }
