// Package filewatch adds filesystem event notifications to typedcfg.
package filewatch

import (
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/nextpkg/typedcfg"
)

// Option watches the parent directory of path, surviving atomic file replacement.
// Use the same path passed to typedcfg.Load. Bursts are debounced for 50ms;
// typedcfg's periodic refresh remains enabled as a fallback.
func Option(path string) typedcfg.Option {
	return typedcfg.WithSubscription(func(notify func(error)) (func() error, error) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		w, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, err
		}
		if err = w.Add(filepath.Dir(abs)); err != nil {
			return nil, errors.Join(err, w.Close())
		}
		cancel := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			timer := time.NewTimer(time.Hour)
			timer.Stop()
			defer timer.Stop()
			var tick <-chan time.Time
			for {
				select {
				case <-cancel:
					return
				case event, ok := <-w.Events:
					if !ok {
						return
					}
					if filepath.Clean(event.Name) == abs && event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
						timer.Reset(50 * time.Millisecond)
						tick = timer.C
					}
				case err, ok := <-w.Errors:
					if !ok {
						return
					}
					notify(err)
				case <-tick:
					tick = nil
					notify(nil)
				}
			}
		}()
		var once sync.Once
		var closeErr error
		return func() error { once.Do(func() { close(cancel); closeErr = w.Close(); <-done }); return closeErr }, nil
	})
}
