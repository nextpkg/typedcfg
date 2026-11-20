package typedcfg

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStrictJSONReload(t *testing.T) {
	type item struct {
		Count int `json:"count"`
	}
	type config struct {
		Enabled bool                       `json:"enabled" default:"true"`
		Port    int                        `json:"port" default:"8080"`
		Items   []item                     `json:"items"`
		Extra   map[string]json.RawMessage `json:"extra"`
	}
	data := `{"enabled":false,"items":[{"count":1},{"COUNT":2}],"extra":{"free":1e400}}`
	source := func(context.Context) ([]byte, error) { return []byte(data), nil }
	cm, err := LoadSource[config](t.Context(), source, WithStrictJSON())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	want := cm.Get()
	require.False(t, want.Enabled)
	require.Equal(t, 8080, want.Port)
	require.Equal(t, "1e400", string(want.Extra["free"]))
	require.Equal(t, []item{{1}, {2}}, want.Items)

	for _, invalid := range []string{
		`{"port":1,"port":2}`,
		`{"items":[{"count":1,"\u0063ount":2}]}`,
		`{"extra":{"free":{"x":1,"x":2}}}`,
		`{"items":[{"typo":1}]}`,
		`{"typo":1}`,
		`{"port":80} {}`,
		`{"port":`,
	} {
		t.Run(invalid, func(t *testing.T) {
			data = invalid
			_, err := LoadSource[config](t.Context(), source, WithStrictJSON())
			require.ErrorIs(t, err, &ConfigError{Type: ErrorTypeParseFailure})
			require.ErrorIs(t, cm.Reload(), &ConfigError{Type: ErrorTypeParseFailure})
			require.Equal(t, want, cm.Get())
		})
	}
}

type customJSONFields struct {
	Value int
}

func (fields *customJSONFields) UnmarshalJSON(data []byte) error {
	var wire struct {
		Free int `json:"free"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	fields.Value = wire.Free
	return nil
}

func TestStrictJSONCustomUnmarshaler(t *testing.T) {
	type config struct {
		Custom customJSONFields `json:"custom"`
	}

	data := `{"custom":{"free":1}}`
	source := func(context.Context) ([]byte, error) { return []byte(data), nil }
	cm, err := LoadSource[config](t.Context(), source, WithStrictJSON())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	require.Equal(t, 1, cm.Get().Custom.Value)

	data = `{"custom":{"free":1,"\u0066ree":2}}`
	err = cm.Reload()
	require.ErrorIs(t, err, &ConfigError{Type: ErrorTypeParseFailure})
	require.ErrorContains(t, err, "duplicate JSON object key ending at byte offset 31")
}

func TestStrictJSONFileOption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	strict, err := Load[liveConfig](path, WithStrictJSON(), WithCreateIfMissing())
	require.NoError(t, err)
	require.Equal(t, 8080, strict.Get().Port)
	require.NoError(t, strict.Close())
	require.NoError(t, os.WriteFile(path, []byte(`{"port":1,"port":2,"extra":3}`), 0600))
	_, err = Load[liveConfig](path, WithStrictJSON())
	require.ErrorIs(t, err, &ConfigError{Type: ErrorTypeParseFailure})
	permissive, err := Load[liveConfig](path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, permissive.Close()) })
	require.Equal(t, 2, permissive.Get().Port)
}
