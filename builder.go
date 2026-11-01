package typedcfg

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type source struct {
	provider koanf.Provider
	parser   koanf.Parser
	path     string
}

// Builder configures ordered sources. Later sources override earlier ones.
// Configure it before Build; concurrent mutation is not supported.
type Builder[T any] struct {
	sources []source
	watch   bool
	tag     string
	err     error
}

func NewBuilder[T any]() *Builder[T] { return &Builder[T]{tag: "json"} }

// AddFile adds a JSON or YAML file. Unsupported extensions fail at Build.
func (b *Builder[T]) AddFile(path string) *Builder[T] {
	var parser koanf.Parser
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		parser = json.Parser()
	case ".yml", ".yaml":
		parser = yaml.Parser()
	default:
		b.err = fmt.Errorf("unsupported configuration extension: %s", path)
		return b
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		b.err = err
		return b
	}
	b.sources = append(b.sources, source{provider: file.Provider(abs), parser: parser, path: abs})
	return b
}

// AddEnv strips prefix, lowercases keys, and maps double underscores to dots.
// APP_DATABASE__MAX_CONNS maps to database.max_conns; single underscores remain literal.
func (b *Builder[T]) AddEnv(prefix string) *Builder[T] {
	return b.AddProvider(env.ProviderWithValue(prefix, ".", func(key, value string) (string, any) {
		return strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(key, prefix), "__", ".")), value
	}), nil)
}

// AddProvider pairs a provider with an explicit parser; nil uses provider.Read().
// Custom sources are re-read on Reload but are not automatically watched.
func (b *Builder[T]) AddProvider(provider koanf.Provider, parser koanf.Parser) *Builder[T] {
	if provider == nil || isNil(provider) {
		b.err = fmt.Errorf("configuration provider is nil")
		return b
	}
	b.sources = append(b.sources, source{provider: provider, parser: parser})
	return b
}

func isNil(v any) bool {
	switch reflect.ValueOf(v).Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflect.ValueOf(v).IsNil()
	}
	return false
}

// WithTag selects one decoding tag for all sources (default: json).
// Use "koanf" for existing koanf-tagged structs, including squash tags.
func (b *Builder[T]) WithTag(tag string) *Builder[T] {
	if tag == "" {
		b.err = fmt.Errorf("decoding tag must not be empty")
	} else {
		b.tag = tag
	}
	return b
}

// WithWatch watches only files added via AddFile. Build fails if there are none
// or if a directory cannot be watched. The Build context bounds watcher lifetime.
func (b *Builder[T]) WithWatch() *Builder[T] { b.watch = true; return b }

func (b *Builder[T]) Build(ctx context.Context) (*ConfigManager[T], error) {
	if b.err != nil {
		return nil, b.err
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if reflect.TypeFor[T]().Kind() != reflect.Struct {
		return nil, fmt.Errorf("configuration type must be a struct")
	}
	ctx, cancel := context.WithCancel(ctx)
	cm := &ConfigManager[T]{sources: append([]source(nil), b.sources...), tag: b.tag, ctx: ctx, cancel: cancel, errors: make(chan error, 1)}
	if b.watch {
		if err := cm.prepareWatch(); err != nil {
			cancel()
			return nil, err
		}
	}
	if err := cm.Reload(); err != nil {
		if cm.watcher != nil {
			_ = cm.watcher.Close()
		}
		cancel()
		return nil, err
	}
	if cm.watcher != nil {
		cm.wg.Add(1)
		go cm.watchFiles()
	}
	return cm, nil
}

func (b *Builder[T]) MustBuild() *ConfigManager[T] {
	cm, err := b.Build(context.Background())
	if err != nil {
		panic(err)
	}
	return cm
}
