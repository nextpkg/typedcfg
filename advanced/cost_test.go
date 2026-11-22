package advanced

import (
	"testing"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/rawbytes"
)

func BenchmarkProviderReload(b *testing.B) {
	cm, err := NewBuilder[testConfig]().AddProvider(rawbytes.Provider([]byte(`{"port":8080}`)), json.Parser()).Build(b.Context())
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
