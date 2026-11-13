package validator

import (
	"testing"

	"github.com/nextpkg/typedcfg"
)

func TestOptInTags(t *testing.T) {
	type config struct {
		Port int `validate:"min=1"`
	}
	if cm, err := typedcfg.Load[config]("", Option()); err == nil {
		_ = cm.Close()
		t.Fatal("tag ignored")
	}
}
