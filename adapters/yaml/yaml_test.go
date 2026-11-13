package yaml

import (
	"context"
	"strings"
	"testing"

	"github.com/nextpkg/typedcfg"
)

func TestDirectDecode(t *testing.T) {
	type config struct {
		Port int    `yaml:"listen_port" default:"8080"`
		Name string `yaml:"name" default:"service"`
	}
	cm, err := typedcfg.LoadSource[config](t.Context(), func(context.Context) ([]byte, error) { return []byte("listen_port: 9000\n"), nil }, Option())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cm.Close(); err != nil {
			t.Error(err)
		}
	}()
	if cm.Get().Port != 9000 || cm.Get().Name != "service" {
		t.Fatal(cm.Get())
	}
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
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("got %v, want error containing %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := cm.Close(); err != nil {
					t.Error(err)
				}
			}()
			cfg := cm.Get()
			if tc.name == "valid" {
				if cfg.Port != 0 || cfg.TLS.Enabled || cfg.Labels["arbitrary"] != "value" {
					t.Fatalf("explicit values lost: %+v", cfg)
				}
			} else if cfg.Port != 8080 || !cfg.TLS.Enabled {
				t.Fatalf("defaults lost: %+v", cfg)
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
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cm.Close(); err != nil {
			t.Error(err)
		}
	}()
	pinned := cm.Get()
	for _, source = range []string{"port: 9001\nunknown: true\n", "port: 9001\n---\nport: 9002\n"} {
		if err := cm.Reload(); err == nil {
			t.Fatal("invalid update accepted")
		}
		if cm.Get() != pinned || pinned.Port != 9000 {
			t.Fatal("invalid update changed the snapshot")
		}
	}
	source = "port: 9003\n"
	if err := cm.Reload(); err != nil {
		t.Fatal(err)
	}
	if cm.Get().Port != 9003 || pinned.Port != 9000 {
		t.Fatal("repaired update did not preserve the pinned version")
	}
}

func TestOptionRemainsPermissive(t *testing.T) {
	type config struct {
		Port int `yaml:"port"`
	}
	cm, err := typedcfg.LoadSource[config](t.Context(), func(context.Context) ([]byte, error) {
		return []byte("port: 9000\nunknown: true\n---\nport: 9001\n"), nil
	}, Option())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cm.Close(); err != nil {
			t.Error(err)
		}
	}()
	if cm.Get().Port != 9000 {
		t.Fatal(cm.Get())
	}
}
