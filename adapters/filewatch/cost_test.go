package filewatch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nextpkg/typedcfg"
)

func BenchmarkWatchOpenClose(b *testing.B) {
	path := filepath.Join(b.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"Port":8080}`), 0600); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		cm, err := typedcfg.Load[struct{ Port int }](path, Option(path))
		if err != nil {
			b.Fatal(err)
		}
		if err := cm.Close(); err != nil {
			b.Fatal(err)
		}
	}
}
