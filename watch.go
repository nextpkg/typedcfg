package typedcfg

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

func (cm *ConfigManager[T]) prepareWatch() error {
	cm.files = make(map[string]struct{})
	dirs := make(map[string]struct{})
	for _, src := range cm.sources {
		if src.path != "" {
			cm.files[src.path] = struct{}{}
			dirs[filepath.Dir(src.path)] = struct{}{}
		}
	}
	if len(dirs) == 0 {
		return fmt.Errorf("WithWatch requires at least one AddFile source")
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	for dir := range dirs {
		if err := watcher.Add(dir); err != nil {
			_ = watcher.Close()
			return fmt.Errorf("watch %s: %w", dir, err)
		}
	}
	cm.watcher = watcher
	return nil
}

func (cm *ConfigManager[T]) watchFiles() {
	defer cm.wg.Done()
	defer cm.closeErrors()
	defer func() { cm.closeErr = cm.watcher.Close() }()
	// Debounce bursts from editors; the parent directory watch survives atomic replacement.
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()
	var tick <-chan time.Time
	for {
		select {
		case <-cm.ctx.Done():
			return
		case event, ok := <-cm.watcher.Events:
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
		case err, ok := <-cm.watcher.Errors:
			if !ok {
				return
			}
			cm.report(err)
		case <-tick:
			tick = nil
			if err := cm.Reload(); err != nil && cm.ctx.Err() == nil {
				cm.report(err)
			}
		}
	}
}
