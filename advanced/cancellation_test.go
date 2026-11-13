package advanced

import (
	"context"
	"errors"
	"testing"

	core "github.com/nextpkg/typedcfg"
	"github.com/stretchr/testify/require"
)

type canceledReloadConfig struct {
	Port int `json:"port"`
}
type canceledReloadManager interface {
	Get() *canceledReloadConfig
	Reload() error
	Close() error
}
type callbackSource struct {
	read func() (map[string]any, error)
}

func (s callbackSource) Read() (map[string]any, error) { return s.read() }

func (s callbackSource) ReadBytes() ([]byte, error) {
	return nil, errors.New("callback source requires a nil parser")
}

func TestCanceledReloadKeepsSnapshot(t *testing.T) {
	for _, kind := range []string{"core", "advanced"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			reloading := false
			var cm canceledReloadManager
			var err error
			if kind == "core" {
				cm, err = core.LoadSource[canceledReloadConfig](ctx, func(context.Context) ([]byte, error) {
					if reloading {
						cancel()
						return []byte(`{"port":9001}`), nil
					}
					return []byte(`{"port":9000}`), nil
				})
			} else {
				cm, err = NewBuilder[canceledReloadConfig]().AddProvider(callbackSource{read: func() (map[string]any, error) {
					if reloading {
						cancel()
						return map[string]any{"port": 9001}, nil
					}
					return map[string]any{"port": 9000}, nil
				}}, nil).Build(ctx)
			}
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, cm.Close()) })
			old := cm.Get()
			reloading = true
			require.ErrorIs(t, cm.Reload(), context.Canceled)
			require.Same(t, old, cm.Get())
			require.Equal(t, 9000, old.Port)
		})
	}
}
