package advanced

import (
	"strconv"
	"testing"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/rawbytes"
)

func BenchmarkProviderReload(b *testing.B) {
	for _, count := range []int{1, 3} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			builder := NewBuilder[testConfig]()
			for i := range count {
				builder.AddProvider(rawbytes.Provider([]byte(`{"port":`+strconv.Itoa(8080+i)+`}`)), json.Parser())
			}
			cm, err := builder.Build(b.Context())
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
		})
	}
}
