package lookup_test

import (
	"testing"
	"time"

	"github.com/nextpkg/typedcfg"
	"github.com/nextpkg/typedcfg/lookup"
	"github.com/stretchr/testify/require"
)

func TestLookupReload(t *testing.T) {
	type config struct {
		Delay   time.Duration `json:"DELAY"`
		Count   int8          `json:"COUNT"`
		Ignored string
	}
	data := map[string]string{"DELAY": "2s", "Ignored": "private"}
	source, err := lookup.Source[config](func(key string) (string, bool) {
		require.Contains(t, []string{"DELAY", "COUNT"}, key)
		v, ok := data[key]
		return v, ok
	})
	require.NoError(t, err)
	manager, err := typedcfg.LoadSource[config](t.Context(), source)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, manager.Close()) })
	require.Equal(t, config{Delay: 2 * time.Second}, *manager.Get())
	data["COUNT"] = "128"
	require.Error(t, manager.Reload())
	require.Equal(t, config{Delay: 2 * time.Second}, *manager.Get())
	data["COUNT"] = "7"
	require.NoError(t, manager.Reload())
	require.Equal(t, int8(7), manager.Get().Count)
}
