package validator

import (
	"testing"
)

func BenchmarkValidatePort(b *testing.B) {
	cfg := struct {
		Port int `validate:"min=1"`
	}{8080}
	for b.Loop() {
		if err := Validate(&cfg); err != nil {
			b.Fatal(err)
		}
	}
}
