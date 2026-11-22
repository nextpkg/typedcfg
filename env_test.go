package typedcfg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNestedEnvReload(t *testing.T) {
	type config struct{ Paths map[string][]string }
	t.Setenv("CFG_DIR", "app")
	source := func(context.Context) ([]byte, error) {
		return []byte(`{"Paths":{"${CFG_DIR}":["/srv/${CFG_DIR}/logs"]}}`), nil
	}
	cm, err := LoadSource[config](t.Context(), source, WithEnvExpansion(true))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	old := cm.Get()
	require.Equal(t, []string{"/srv/app/logs"}, old.Paths["${CFG_DIR}"])
	t.Setenv("CFG_DIR", "")
	require.Error(t, cm.Reload())
	require.Same(t, old, cm.Get())
}

func TestExpandedSnapshotIsolatedFromRetainedReferences(t *testing.T) {
	type config struct{ Names map[*string]string }
	key := "original"
	var retained *config
	decode := func(_ []byte, dst any) error {
		dst.(*config).Names = map[*string]string{&key: "value"}
		return nil
	}
	for _, validate := range []bool{false, true} {
		opts := []Option{WithEnvExpansion(true), WithCodec(decode, nil)}
		if validate {
			opts = append(opts, WithValidation(func(v any) error { retained = v.(*config); return nil }))
		}
		key = "original"
		cm, err := LoadSource[config](t.Context(), func(context.Context) ([]byte, error) { return nil, nil }, opts...)
		require.NoError(t, err)
		key = "changed"
		if validate {
			clear(retained.Names)
		}
		require.Len(t, cm.Get().Names, 1)
		for k := range cm.Get().Names {
			require.Equal(t, "original", *k)
		}
		require.NoError(t, cm.Close())
	}
}

type retainingEnvConfig struct{ Names map[string]string }

var retainedByMethod *retainingEnvConfig

func (cfg *retainingEnvConfig) Validate() error {
	retainedByMethod = cfg
	return nil
}

func TestExpandedSnapshotAfterValidation(t *testing.T) {
	t.Setenv("CFG_NAME", "original")
	source := func(context.Context) ([]byte, error) { return []byte(`{"Names":{"name":"${CFG_NAME}"}}`), nil }
	t.Run("callback", func(t *testing.T) {
		type config retainingEnvConfig
		var retained *config
		cm, err := LoadSource[config](t.Context(), source, WithEnvExpansion(true), WithValidation(func(v any) error { retained = v.(*config); return nil }))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, cm.Close()) })
		retained.Names["name"] = "changed"
		require.Equal(t, "original", cm.Get().Names["name"])
	})
	t.Run("method", func(t *testing.T) {
		t.Cleanup(func() { retainedByMethod = nil })
		cm, err := LoadSource[retainingEnvConfig](t.Context(), source, WithEnvExpansion(true))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, cm.Close()) })
		retainedByMethod.Names["name"] = "changed"
		require.Equal(t, "original", cm.Get().Names["name"])
	})
}
