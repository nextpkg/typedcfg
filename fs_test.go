package typedcfg

import (
	"embed"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

//go:embed testdata/embedded_config.json
var embeddedConfig embed.FS

func TestLoadFSWithEmbeddedConfig(t *testing.T) {
	type config struct {
		Enabled bool `json:"enabled" default:"true"`
		Port    int  `json:"port" default:"8080"`
	}

	cm, err := LoadFS[config](t.Context(), embeddedConfig, "testdata/embedded_config.json", WithStrictJSON())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	require.Equal(t, &config{Port: 8080}, cm.Get())
}

func TestLoadFSReload(t *testing.T) {
	type config struct {
		Port int `json:"port"`
	}

	const name = "config.json"
	filesystem := fstest.MapFS{name: {Data: []byte(`{"port":8080}`)}}
	cm, err := LoadFS[config](t.Context(), filesystem, name, WithStrictJSON())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })

	filesystem[name].Data = []byte(`{"port":8081,"typo":true}`)
	require.ErrorIs(t, cm.Reload(), &ConfigError{Type: ErrorTypeParseFailure})
	require.Equal(t, 8080, cm.Get().Port)

	filesystem[name].Data = []byte(`{"port":8081}`)
	require.NoError(t, cm.Reload())
	require.Equal(t, 8081, cm.Get().Port)

	delete(filesystem, name)
	require.ErrorIs(t, cm.Reload(), fs.ErrNotExist)
	require.Equal(t, 8081, cm.Get().Port)

	_, err = LoadFS[config](t.Context(), filesystem, "../config.json")
	require.ErrorIs(t, err, fs.ErrInvalid)
}
