package typedcfg

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/knadh/koanf/v2"
	"github.com/nextpkg/typedcfg/defaults"
	"github.com/nextpkg/typedcfg/validator"
)

var ErrClosed = errors.New("configuration manager is closed")

// ConfigManager publishes validated snapshots. Get is safe concurrently with
// Reload and Close. Callers must not mutate snapshots, including nested maps/slices.
type ConfigManager[T any] struct {
	sources      []source
	tag          string
	cfg          atomic.Pointer[T]
	mu           sync.Mutex
	closed       bool
	ctx          context.Context
	cancel       context.CancelFunc
	watcher      *fsnotify.Watcher
	changes      chan error
	stops        []func() error
	pollInterval time.Duration
	pollRemote   bool
	files        map[string]struct{}
	wg           sync.WaitGroup
	closeOnce    sync.Once
	errorsOnce   sync.Once
	closeErr     error
	errors       chan error
}

// Get returns the current read-only snapshot; existing snapshots remain valid
// after Reload or Close. A nil/uninitialized manager returns nil.
func (cm *ConfigManager[T]) Get() *T {
	if cm == nil {
		return nil
	}
	return cm.cfg.Load()
}

// Reload re-reads all sources into a fresh configuration. Failures leave the
// previous snapshot intact. Reloads are serialized, including automatic reloads.
func (cm *ConfigManager[T]) Reload() error {
	if cm == nil {
		return ErrClosed
	}
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.closed || cm.ctx == nil {
		return ErrClosed
	}
	if err := cm.ctx.Err(); err != nil {
		return err
	}
	k := koanf.New(".")
	for _, src := range cm.sources {
		provider := src.provider
		if remote, ok := provider.(*urlProvider); ok {
			bound := *remote
			bound.ctx = cm.ctx
			provider = &bound
		}
		if err := k.Load(provider, src.parser); err != nil {
			name := src.path
			if name == "" {
				name = fmt.Sprintf("%T", src.provider)
			}
			return NewParseError(name, "failed to load source", err)
		}
	}
	var cfg T
	if err := defaults.SetDefaults(&cfg); err != nil {
		return NewParseError("defaults", "invalid default", err)
	}
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: cm.tag}); err != nil {
		return NewParseError("decode", "failed to decode configuration", err)
	}
	if err := validator.Validate(&cfg); err != nil {
		return NewValidationError("validator", "invalid configuration", err)
	}
	if err := cm.ctx.Err(); err != nil {
		return err
	}
	cm.cfg.Store(&cfg)
	return nil
}

// Errors reports asynchronous watch/reload failures without logging globally.
// One pending error is buffered; additional errors may be dropped until consumed.
// The channel closes when watching stops or Close returns.
func (cm *ConfigManager[T]) Errors() <-chan error { return cm.errors }

func (cm *ConfigManager[T]) report(err error) {
	select {
	case cm.errors <- err:
	default:
	}
}

func (cm *ConfigManager[T]) closeErrors() { cm.errorsOnce.Do(func() { close(cm.errors) }) }

// Close stops and joins the watcher and prevents subsequent Reload calls.
// It also cancels in-flight AddURL requests.
// Concurrent/repeated calls are safe. Custom provider reads already in progress
// must finish before Close returns; providers own their I/O timeouts/resources.
func (cm *ConfigManager[T]) Close() error {
	if cm == nil {
		return nil
	}
	cm.closeOnce.Do(func() {
		if cm.cancel != nil {
			cm.cancel()
		}
		cm.mu.Lock()
		cm.closed = true
		cm.mu.Unlock()
		cm.wg.Wait()
		if cm.errors != nil {
			cm.closeErrors()
		}
	})
	return cm.closeErr
}
