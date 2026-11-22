package demo

import (
	"context"
	"encoding/json"
	"fmt"

	"strings"
	"testing"

	"github.com/nextpkg/typedcfg"
	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

type settings struct {
	Path string `json:"path" yaml:"path" toml:"path"`
}

func TestFormats(t *testing.T) {
	const template = "${DEMO_DIR}/logs"
	const dir = "a\"b\n#c"
	t.Setenv("DEMO_DIR", dir)
	for _, tc := range []struct {
		name, input string
		decode      typedcfg.Decoder
	}{
		{"json", `{"path":"${DEMO_DIR}/logs"}`, json.Unmarshal},
		{"yaml", `path: "${DEMO_DIR}/logs"`, yaml.Unmarshal},
		{"toml", `path = "${DEMO_DIR}/logs"`, toml.Unmarshal},
	} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/enabled=%t", tc.name, enabled), func(t *testing.T) {
				data := tc.input
				src := func(context.Context) ([]byte, error) { return []byte(data), nil }
				cm, err := typedcfg.LoadSource[settings](t.Context(), src, typedcfg.WithCodec(tc.decode, nil), typedcfg.WithEnvExpansion(enabled))
				if err != nil {
					t.Fatal(err)
				}
				defer cm.Close()
				want := template
				if enabled {
					want = dir + "/logs"
				}
				old := cm.Get()
				if got := old.Path; got != want {
					t.Fatalf("got %q want %q", got, want)
				}
				if enabled {
					data = strings.ReplaceAll(data, "DEMO_DIR", "")
					if cm.Reload() == nil || cm.Get() != old {
						t.Fatal("invalid reload replaced snapshot or returned success")
					}
				}
			})
		}
	}
}
