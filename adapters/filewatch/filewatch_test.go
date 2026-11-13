package filewatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nextpkg/typedcfg"
)

func TestAtomicReplacementNotification(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"Port":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	cm, err := typedcfg.Load[struct{ Port int }](path, Option(path), typedcfg.WithPollInterval(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cm.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := os.WriteFile(path+".new", []byte(`{"Port":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".new", path); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && cm.Get().Port != 2 {
		time.Sleep(time.Millisecond)
	}
	if cm.Get().Port != 2 {
		t.Fatal("event not delivered")
	}
}
