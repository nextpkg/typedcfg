package typedcfg

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func BenchmarkMapReload(b *testing.B) {
	type config struct {
		Labels map[string]string
		Names  []string `default:"a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p"`
	}
	values := map[string]string{}
	for i := range 256 {
		values[fmt.Sprint(i)] = "value"
	}
	data, err := json.Marshal(struct{ Labels map[string]string }{values})
	if err != nil {
		b.Fatal(err)
	}
	src := func(context.Context) ([]byte, error) { return data, nil }
	for _, tc := range []struct {
		name string
		opts []Option
	}{{"plain", nil}, {"strict", []Option{WithStrictJSON()}}, {"env", []Option{WithEnvExpansion(true)}}} {
		b.Run(tc.name, func(b *testing.B) {
			cm, err := LoadSource[config](b.Context(), src, tc.opts...)
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
