// Package typedcfg loads single-source configurations into atomic snapshots.
// JSON, HTTP, polling, defaults and custom validation use only the standard library.
package typedcfg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/nextpkg/typedcfg/defaults"
	"github.com/nextpkg/typedcfg/internal/fileutil"
	"github.com/nextpkg/typedcfg/internal/httpconfig"
)

// Decoder decodes into a pointer to the configuration, already populated with defaults.
type Decoder func([]byte, any) error
type Encoder func(any) ([]byte, error)

// Source reads one complete configuration; it must honor cancellation or bound its I/O.
type Source func(context.Context) ([]byte, error)

// Subscribe registers change notifications and returns an idempotent cleanup function.
// It must return promptly, clean up on registration failure and never mutate snapshots.
type Subscribe func(notify func(error)) (stop func() error, err error)
type Option func(*options)
type options struct {
	decode        Decoder
	encode        Encoder
	validate      func(any) error
	subscribe     Subscribe
	watch, create bool
	interval      time.Duration
	http          httpconfig.Options
	err           error
}

// WithWatch enables periodic refresh (files: 1s, HTTP: 30s).
// For filesystem events, use the optional filewatch adapter instead.
func WithWatch() Option { return func(o *options) { o.watch = true } }
func WithPollInterval(d time.Duration) Option {
	return func(o *options) {
		if d <= 0 {
			o.err = errors.New("poll interval must be positive")
		} else {
			o.interval = d
		}
	}
}

// WithSubscription enables notifications plus periodic refresh as a fallback.
func WithSubscription(s Subscribe) Option {
	return func(o *options) {
		if s == nil {
			o.err = errors.New("subscription is nil")
		} else {
			o.subscribe = s
			o.watch = true
		}
	}
}

// WithCodec selects direct decoding and optional encoding for default-file creation.
// Core JSON uses json tags; other codecs define their own tag and conversion rules.
func WithCodec(decode Decoder, encode Encoder) Option {
	return func(o *options) {
		if decode == nil {
			o.err = errors.New("decoder is nil")
		} else {
			o.decode = decode
			o.encode = encode
		}
	}
}

// WithValidation adds validation before the configuration's own Validate method.
// Use the optional validator adapter to enable validate tags.
func WithValidation(fn func(any) error) Option {
	return func(o *options) {
		if fn == nil {
			o.err = errors.New("validator is nil")
		} else {
			o.validate = fn
		}
	}
}
func WithCreateIfMissing() Option { return func(o *options) { o.create = true } }

// HTTPOptions controls bounded HTTP reads; headers and the client are copied at Load.
type HTTPOptions = httpconfig.Options

func WithHTTP(opts HTTPOptions) Option { return func(o *options) { o.http = opts } }

func configure(opts []Option) (options, error) {
	o := options{decode: json.Unmarshal, encode: json.Marshal, interval: time.Second}
	for _, apply := range opts {
		if apply == nil {
			return o, errors.New("option is nil")
		}
		apply(&o)
	}
	return o, o.err
}

// Load reads one JSON file or HTTP(S) URL. An empty path loads only defaults.
// Use LoadContext to bind refresh to an application context and Close to stop it.
func Load[T any](path string, opts ...Option) (*ConfigManager[T], error) {
	return LoadContext[T](context.Background(), path, opts...)
}
func MustLoad[T any](path string, opts ...Option) *ConfigManager[T] {
	cm, err := Load[T](path, opts...)
	if err != nil {
		panic(err)
	}
	return cm
}
func LoadContext[T any](ctx context.Context, path string, opts ...Option) (*ConfigManager[T], error) {
	remote := strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
	if remote {
		opts = append([]Option{WithPollInterval(30 * time.Second)}, opts...)
	}
	o, err := configure(opts)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if reflect.TypeFor[T]().Kind() != reflect.Struct {
		return nil, errors.New("configuration type must be a struct")
	}
	var source Source
	switch {
	case remote:
		if o.create {
			return nil, errors.New("default-file creation requires a local file")
		}
		reader, err := httpconfig.New(path, o.http)
		if err != nil {
			return nil, err
		}
		source = reader.Read
	case path == "":
		if o.create || o.watch {
			return nil, errors.New("creation and watching require a source")
		}
		source = func(context.Context) ([]byte, error) { return []byte("{}"), nil }
	default:
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		if o.create {
			if _, err = os.Lstat(abs); errors.Is(err, os.ErrNotExist) {
				if o.encode == nil {
					return nil, errors.New("default-file creation requires an encoder")
				}
				var cfg T
				if err = defaults.SetDefaults(&cfg); err != nil {
					return nil, err
				}
				data, err := o.encode(&cfg)
				if err != nil {
					return nil, err
				}
				if err = ctx.Err(); err != nil {
					return nil, err
				}
				if err = fileutil.CreateExclusive(abs, data); err != nil {
					return nil, err
				}
			} else if err != nil {
				return nil, err
			}
		}
		source = func(ctx context.Context) ([]byte, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return os.ReadFile(abs)
		}
	}
	return open[T](ctx, source, o)
}

// LoadSource loads a caller-owned source. It does not close that source.
func LoadSource[T any](ctx context.Context, source Source, opts ...Option) (*ConfigManager[T], error) {
	o, err := configure(opts)
	if err != nil {
		return nil, err
	}
	if o.create {
		return nil, fmt.Errorf("default-file creation requires LoadContext with a file")
	}
	return open[T](ctx, source, o)
}
