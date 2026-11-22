package yaml

import (
	"context"
	"testing"

	"github.com/nextpkg/typedcfg"
)

func BenchmarkYAMLReload(b *testing.B) {
	cm, err := typedcfg.LoadSource[struct{ Port int }](b.Context(), func(context.Context) ([]byte, error) { return []byte("port: 8080"), nil }, Option())
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := cm.Close(); err != nil {
			b.Fatal(err)
		}
	})
	for b.Loop() {
		if err := cm.Reload(); err != nil {
			b.Fatal(err)
		}
	}
}
