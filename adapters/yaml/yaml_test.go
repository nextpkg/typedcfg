package yaml

import (
	"context"
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
