package typedcfg

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nextpkg/typedcfg/defaults"
)

var ErrClosed = errors.New("configuration manager is closed")

// ConfigManager publishes validated snapshots. Get's entire object graph is read-only.
type ConfigManager[T any] struct {
	cfg      atomic.Pointer[T]
	source   Source
	opts     options
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	closed   bool
	once     sync.Once
	wg       sync.WaitGroup
	errors   chan error
	closeErr error
}

func open[T any](ctx context.Context, source Source, o options) (*ConfigManager[T], error) {
	if ctx == nil || source == nil {
		return nil, errors.New("context and source are required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if reflect.TypeFor[T]().Kind() != reflect.Struct {
		return nil, errors.New("configuration type must be a struct")
	}
	ctx, cancel := context.WithCancel(ctx)
	cm := &ConfigManager[T]{source: source, opts: o, ctx: ctx, cancel: cancel, errors: make(chan error, 1)}
	changes := make(chan error, 1)
	var stop func() error
	if o.subscribe != nil {
		var err error
		stop, err = o.subscribe(func(err error) {
			select {
			case changes <- err:
			default:
			}
		})
		if err != nil {
			cancel()
			return nil, err
		}
		if stop == nil {
			cancel()
			return nil, errors.New("subscription returned a nil stop function")
		}
	}
	if err := cm.Reload(); err != nil {
		cancel()
		if stop != nil {
			err = errors.Join(err, stop())
		}
		return nil, err
	}
	if o.watch {
		cm.wg.Add(1)
		go func() {
			defer cm.wg.Done()
			defer close(cm.errors)
			if stop != nil {
				defer func() { cm.closeErr = stop() }()
			}
			ticker := time.NewTicker(o.interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case err := <-changes:
					if err != nil {
						cm.report(err)
					}
				case <-ticker.C:
				}
				if err := cm.Reload(); err != nil && ctx.Err() == nil {
					cm.report(err)
				}
			}
		}()
	}
	return cm, nil
}

// Get returns the shared snapshot without copying. Do not modify any reachable
// fields, slices, maps or pointers; use Snapshot for a mutable private copy.
func (cm *ConfigManager[T]) Get() *T {
	if cm == nil {
		return nil
	}
	return cm.cfg.Load()
}

// Snapshot deep-copies the current version for independent local modification.
func (cm *ConfigManager[T]) Snapshot() *T {
	if cm == nil {
		return nil
	}
	value, err := snapshotCopy(cm.cfg.Load())
	if err != nil {
		panic("typedcfg: invalid internal snapshot: " + err.Error())
	}
	return value
}

// Reload builds a fresh object and atomically publishes only after validation.
// Manual and automatic reloads are serialized. Errors preserve the previous version.
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
	data, err := cm.source(cm.ctx)
	if err != nil {
		return NewParseError("source", "failed to load", err)
	}
	var cfg T
	if err = defaults.SetDefaults(&cfg); err != nil {
		return NewParseError("defaults", "invalid default", err)
	}
	if err = cm.opts.decode(data, &cfg); err != nil {
		return NewParseError("decode", "invalid configuration", err)
	}
	if cm.opts.validate != nil {
		if err = cm.opts.validate(&cfg); err != nil {
			return NewValidationError("validator", "invalid configuration", err)
		}
	}
	if v, ok := any(&cfg).(interface{ Validate() error }); ok {
		if err = v.Validate(); err != nil {
			return NewValidationError("validator", "invalid configuration", err)
		}
	}
	// Detach retained decoder/validator references once per successful publication.
	snapshot, err := snapshotCopy(&cfg)
	if err != nil {
		return NewParseError("snapshot", "configuration cannot be isolated", err)
	}
	if err = cm.ctx.Err(); err != nil {
		return err
	}
	cm.cfg.Store(snapshot)
	return nil
}
func (cm *ConfigManager[T]) report(err error) {
	select {
	case cm.errors <- err:
	default:
	}
}

// Errors buffers one asynchronous error; additional errors may be dropped.
func (cm *ConfigManager[T]) Errors() <-chan error { return cm.errors }

// Close cancels in-flight HTTP reads, joins subscriptions and retains the last snapshot.
func (cm *ConfigManager[T]) Close() error {
	if cm == nil {
		return nil
	}
	cm.once.Do(func() {
		if cm.cancel != nil {
			cm.cancel()
		}
		cm.mu.Lock()
		cm.closed = true
		cm.mu.Unlock()
		cm.wg.Wait()
		if !cm.opts.watch && cm.errors != nil {
			close(cm.errors)
		}
	})
	return cm.closeErr
}
