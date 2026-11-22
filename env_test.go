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
