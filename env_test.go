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
