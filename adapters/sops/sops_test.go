package sops_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nextpkg/typedcfg"
	"github.com/nextpkg/typedcfg/adapters/sops"
	"github.com/nextpkg/typedcfg/adapters/yaml"
	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"
)

type config struct {
	Name string `json:"name" yaml:"name" toml:"name"`
}

func TestLoad(t *testing.T) {
	t.Setenv("SOPS_AGE_KEY_FILE", "testdata/age.txt")

	tests := []struct {
		name   string
		format string
		opts   []typedcfg.Option
	}{
		{name: "json", format: "json"},
		{name: "yaml", opts: []typedcfg.Option{yaml.Option()}},
		{name: "toml", format: "binary", opts: []typedcfg.Option{typedcfg.WithCodec(toml.Unmarshal, toml.Marshal)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join("testdata", "config.enc."+test.name)
			cm, err := typedcfg.LoadSource[config](t.Context(), sops.File(path, test.format), test.opts...)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, cm.Close()) })
			require.Equal(t, test.name, cm.Get().Name)
		})
	}
}

func TestReloadPreservesSnapshotOnDecryptionFailure(t *testing.T) {
	t.Setenv("SOPS_AGE_KEY_FILE", "testdata/age.txt")

	path := filepath.Join(t.TempDir(), "config.enc.json")
	initial, err := os.ReadFile("testdata/config.enc.json")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, initial, 0o600))

	cm, err := typedcfg.LoadSource[config](t.Context(), sops.File(path, "json"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })

	snapshot := cm.Get()
	require.Equal(t, "json", snapshot.Name)

	require.NoError(t, os.WriteFile(path, []byte(`{"name":"missing SOPS metadata"}`), 0o600))
	require.Error(t, cm.Reload())
	require.Same(t, snapshot, cm.Get())

	updated, err := os.ReadFile("testdata/updated.enc.json")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, updated, 0o600))
	require.NoError(t, cm.Reload())
	require.Equal(t, "updated", cm.Get().Name)
	require.Equal(t, "json", snapshot.Name)
}
