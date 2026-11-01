// Package yaml adds direct YAML decoding to typedcfg.
package yaml

import (
	"github.com/nextpkg/typedcfg"
	"go.yaml.in/yaml/v3"
)

// Option uses yaml tags and yaml.v3 decoding rules (including duration strings).
func Option() typedcfg.Option { return typedcfg.WithCodec(yaml.Unmarshal, yaml.Marshal) }
