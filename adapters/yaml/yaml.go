// Package yaml adds direct YAML decoding to typedcfg.
package yaml

import (
	"bytes"
	"errors"
	"io"

	"github.com/nextpkg/typedcfg"
	"go.yaml.in/yaml/v3"
)

// Option uses yaml tags and yaml.v3 decoding rules (including duration strings).
func Option() typedcfg.Option { return typedcfg.WithCodec(yaml.Unmarshal, yaml.Marshal) }

// StrictOption uses YAML decoding with unknown struct fields and additional
// documents rejected. Empty input leaves defaults intact, as with Option.
func StrictOption() typedcfg.Option { return typedcfg.WithCodec(decodeStrict, yaml.Marshal) }

func decodeStrict(data []byte, target any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return errors.New("multiple YAML documents are not allowed")
	}
	return nil
}
