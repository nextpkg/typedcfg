package typedcfg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// WithStrictJSON selects the JSON codec and rejects duplicate object keys
// and unknown struct fields. Struct names follow encoding/json matching rules.
func WithStrictJSON() Option {
	return WithCodec(decodeStrictJSON, json.Marshal)
}

func decodeStrictJSON(data []byte, dst any) error {
	// Validate document boundaries and the standard library's nesting limit
	// before recursively visiting tokens. UseNumber also admits large numbers
	// in RawMessage and custom types without first converting to float64.
	if !json.Valid(data) {
		return errors.New("invalid JSON document")
	}
	tokens := json.NewDecoder(bytes.NewReader(data))
	tokens.UseNumber()
	if err := checkObjectKeys(tokens); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

// Consume one complete value. Each object owns its own set of decoded names.
func checkObjectKeys(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('{'):
		seen := make(map[string]struct{})
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name := key.(string) // Valid JSON object member names are strings.
			if _, found := seen[name]; found {
				return fmt.Errorf("duplicate JSON object key ending at byte offset %d", decoder.InputOffset())
			}
			seen[name] = struct{}{}
			if err := checkObjectKeys(decoder); err != nil {
				return err
			}
		}
	case json.Delim('['):
		for decoder.More() {
			if err := checkObjectKeys(decoder); err != nil {
				return err
			}
		}
	default:
		return nil
	}
	_, err = decoder.Token()
	return err
}
