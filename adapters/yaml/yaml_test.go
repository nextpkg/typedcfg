package yaml

import (
	"context"
	"testing"

	"github.com/nextpkg/typedcfg"
	"github.com/stretchr/testify/require"
)

func TestDirectDecode(t *testing.T) {
	type config struct {
		Port int    `yaml:"listen_port" default:"8080"`
		Name string `yaml:"name" default:"service"`
	}
	cm, err := typedcfg.LoadSource[config](t.Context(), func(context.Context) ([]byte, error) { return []byte("listen_port: 9000\n"), nil }, Option())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	require.Equal(t, 9000, cm.Get().Port)
	require.Equal(t, "service", cm.Get().Name)
}

func TestStrictDecode(t *testing.T) {
	type config struct {
		Port int `yaml:"port" default:"8080"`
		TLS  struct {
			Enabled bool `yaml:"enabled" default:"true"`
		} `yaml:"tls"`
		Labels map[string]string `yaml:"labels"`
	}
	for _, tc := range []struct {
		name, source, wantError string
	}{
		{"valid", "port: 0\ntls: {enabled: false}\nlabels: {arbitrary: value}\n", ""},
		{"empty", "", ""},
		{"comments", "# defaults only\n", ""},
		{"unknown field", "porrt: 9000\n", "field porrt not found"},
		{"nested unknown field", "tls: {enable: true}\n", "field enable not found"},
		{"second document", "port: 9000\n---\nport: 9001\n", "multiple YAML documents"},
		{"empty second document", "port: 9000\n---\n", "multiple YAML documents"},
		{"malformed", "port: [\n", "yaml:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cm, err := typedcfg.LoadSource[config](t.Context(), func(context.Context) ([]byte, error) {
				return []byte(tc.source), nil
			}, StrictOption())
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				return
			}
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, cm.Close()) })
			cfg := cm.Get()
			if tc.name == "valid" {
				require.Zero(t, cfg.Port)
				require.False(t, cfg.TLS.Enabled)
				require.Equal(t, "value", cfg.Labels["arbitrary"])
			} else {
				require.Equal(t, 8080, cfg.Port)
				require.True(t, cfg.TLS.Enabled)
			}
		})
	}
}

func TestStrictReloadPreservesLastValidSnapshot(t *testing.T) {
	type config struct {
		Port int `yaml:"port"`
	}
	source := "port: 9000\n"
	cm, err := typedcfg.LoadSource[config](t.Context(), func(context.Context) ([]byte, error) {
		return []byte(source), nil
	}, StrictOption())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	pinned := cm.Get()
	for _, source = range []string{"port: 9001\nunknown: true\n", "port: 9001\n---\nport: 9002\n"} {
		require.Error(t, cm.Reload())
		require.Same(t, pinned, cm.Get())
		require.Equal(t, 9000, pinned.Port)
	}
	source = "port: 9003\n"
	require.NoError(t, cm.Reload())
	require.Equal(t, 9003, cm.Get().Port)
	require.Equal(t, 9000, pinned.Port)
}

func TestOptionRemainsPermissive(t *testing.T) {
	type config struct {
		Port int `yaml:"port"`
	}
	cm, err := typedcfg.LoadSource[config](t.Context(), func(context.Context) ([]byte, error) {
		return []byte("port: 9000\nunknown: true\n---\nport: 9001\n"), nil
	}, Option())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	require.Equal(t, 9000, cm.Get().Port)
}
