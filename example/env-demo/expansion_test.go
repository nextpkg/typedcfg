package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/nextpkg/typedcfg"
	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

type settings struct {
	Path string `json:"path" yaml:"path" toml:"path"`
}

var nameRE = regexp.MustCompile("^[A-Za-z_][A-Za-z0-9_]*$")

func expand(s string) (string, error) {
	var out strings.Builder
	for {
		prefix, tail, found := strings.Cut(s, "${")
		out.WriteString(prefix)
		if !found {
			return out.String(), nil
		}
		name, rest, closed := strings.Cut(tail, "}")
		if !closed || !nameRE.MatchString(name) {
			return "", fmt.Errorf("invalid environment placeholder")
		}
		value, ok := os.LookupEnv(name)
		if !ok {
			return "", fmt.Errorf("unset variable %s", name)
		}
		out.WriteString(value)
		s = rest
	}
}

func codec(decode typedcfg.Decoder, enabled bool) typedcfg.Decoder {
	return func(data []byte, target any) error {
		if err := decode(data, target); err != nil {
			return err
		}
		if enabled {
			cfg := reflect.ValueOf(target).Elem()
			for i := 0; i < cfg.NumField(); i++ {
				field := cfg.Field(i)
				if field.Kind() != reflect.String || !field.CanSet() {
					continue
				}
				expanded, err := expand(field.String())
				if err != nil {
					return err
				}
				field.SetString(expanded)
			}
		}
		return nil
	}
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
				cm, err := typedcfg.LoadSource[settings](t.Context(), src, typedcfg.WithCodec(codec(tc.decode, enabled), nil))
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
