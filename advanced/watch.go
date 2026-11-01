package advanced

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchProvider optionally subscribes a source to change notifications. Watch
// must return promptly; notify must be safe to call concurrently. A nil error
// requests a reload, errors are forwarded to Errors before reloading. The returned
// stop function releases the subscription, not the provider itself. On failure,
// Watch must clean up any partial subscription before returning.
type WatchProvider interface {
	Watch(notify func(error)) (stop func() error, err error)
}

func (cm *ConfigManager[T]) stopWatching() error {
	var err error
	for i := len(cm.stops) - 1; i >= 0; i-- {
		err = errors.Join(err, cm.stops[i]())
	}
	if cm.watcher != nil {
		err = errors.Join(err, cm.watcher.Close())
	}
	return err
}

func (cm *ConfigManager[T]) prepareWatch() error {
	cm.changes = make(chan error, 1)
	cm.files = make(map[string]struct{})
	dirs := make(map[string]struct{})
	for _, src := range cm.sources {
		if src.path != "" {
			cm.files[src.path] = struct{}{}
			dirs[filepath.Dir(src.path)] = struct{}{}
		}
		if _, ok := src.provider.(*urlProvider); ok {
			cm.pollRemote = true
		}
	}
	if len(dirs) > 0 {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return err
		}
		cm.watcher = watcher
		for dir := range dirs {
			if err := watcher.Add(dir); err != nil {
				return fmt.Errorf("watch %s: %w", dir, err)
			}
		}
	}
	for _, src := range cm.sources {
		if provider, ok := src.provider.(WatchProvider); ok {
			stop, err := provider.Watch(func(err error) {
				select {
				case cm.changes <- err:
				default:
				}
			})
			if err != nil {
				return fmt.Errorf("watch %T: %w", src.provider, err)
			}
			if stop == nil {
				return fmt.Errorf("watch %T returned a nil stop function", src.provider)
			}
			cm.stops = append(cm.stops, stop)
			cm.pollRemote = true
		}
	}
	if cm.watcher == nil && !cm.pollRemote && len(cm.stops) == 0 {
		return fmt.Errorf("WithWatch requires an AddFile, AddURL, or WatchProvider source")
	}
	return nil
}

func (cm *ConfigManager[T]) watchSources() {
	defer cm.wg.Done()
	defer cm.closeErrors()
	defer func() { cm.closeErr = cm.stopWatching() }()
	var events <-chan fsnotify.Event
	var watchErrors <-chan error
	if cm.watcher != nil {
		events = cm.watcher.Events
		watchErrors = cm.watcher.Errors
	}
	var poll <-chan time.Time
	if cm.pollRemote {
		ticker := time.NewTicker(cm.pollInterval)
		defer ticker.Stop()
		poll = ticker.C
	}
	reload := func() {
		if err := cm.Reload(); err != nil && cm.ctx.Err() == nil {
			cm.report(err)
		}
	}
	// Debounce bursts from editors; the parent directory watch survives atomic replacement.
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()
	var tick <-chan time.Time
	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-poll:
			reload()
		case err := <-cm.changes:
			if err != nil {
				cm.report(err)
			}
			reload()
		case event, ok := <-events:
			if !ok {
				return
			}
			if _, matches := cm.files[filepath.Clean(event.Name)]; !matches {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}
			timer.Reset(50 * time.Millisecond)
			tick = timer.C
		case err, ok := <-watchErrors:
			if !ok {
				return
			}
			cm.report(err)
		case <-tick:
			tick = nil
			reload()
		}
	}
}
