package typedcfg

import (
	"context"
	"encoding/json"
	"testing"
)

func BenchmarkNestedLifecycle(b *testing.B) {
	value := getCostNested{Services: map[string]*getCostEntry{"api": {Address: "localhost", Weights: make([]int, 1024)}}}
	data, err := json.Marshal(value)
	if err != nil {
		b.Fatal(err)
	}
	cm, err := LoadSource[getCostNested](b.Context(), func(context.Context) ([]byte, error) { return data, nil })
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := cm.Close(); err != nil {
			b.Fatal(err)
		}
	})
	b.Run("Snapshot", func(b *testing.B) {
		for b.Loop() {
			getCostResult = cm.Snapshot()
		}
	})
	b.Run("Reload", func(b *testing.B) {
		for b.Loop() {
			if err := cm.Reload(); err != nil {
				b.Fatal(err)
			}
		}
	})
}
