package typedcfg

import (
	"encoding/json"
	"fmt"
	"testing"
)

var getCostResult any

type getCostSmall struct {
	Port    int
	Enabled bool
	Name    string
}
type getCostEntry struct {
	Address string
	Weights []int
	Labels  map[string]string
}
type getCostNested struct{ Services map[string]*getCostEntry }

func benchmarkGetCost[T any](b *testing.B, value *T) {
	cm := &ConfigManager[T]{}
	cm.cfg.Store(value)
	data, err := json.Marshal(value)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		getCostResult = cm.Get()
	}
	b.ReportMetric(float64(len(data)), "fixture-JSON-bytes")
}
func BenchmarkGetCostSmall(b *testing.B) {
	benchmarkGetCost(b, &getCostSmall{8080, true, "example-service"})
}
func BenchmarkGetCostNested(b *testing.B) {
	for _, count := range []int{20, 200} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			value := &getCostNested{Services: make(map[string]*getCostEntry, count)}
			for i := range count {
				value.Services[fmt.Sprintf("service-%03d", i)] = &getCostEntry{
					Address: fmt.Sprintf("https://service-%03d.example.com", i),
					Weights: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
					Labels:  map[string]string{"region": "cn-shenzhen", "tier": "backend", "team": "example"},
				}
			}
			benchmarkGetCost(b, value)
		})
	}
}
