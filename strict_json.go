package typedcfg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

// WithStrictJSON selects the JSON codec and rejects duplicate object keys
// and unknown struct fields. Struct names follow encoding/json matching rules.
func WithStrictJSON() Option {
	return WithCodec(decodeStrictJSON, json.Marshal)
}

func decodeStrictJSON(data []byte, dst any) error {
	// Validate syntax and nesting depth before scanning for object keys.
	if !json.Valid(data) {
		return errors.New("invalid JSON document")
	}
	if err := checkObjectKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

// In a valid document only object keys can precede a colon. Track each
// object's names while skipping quoted values and scalar array elements.
func checkObjectKeys(data []byte) error {
	var objects []map[string]struct{}
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '{':
			objects = append(objects, make(map[string]struct{}))
		case '}':
			objects = objects[:len(objects)-1]
		case '"':
			start := i
			escaped := false
			for i++; data[i] != '"'; i++ {
				if data[i] == '\\' {
					i++
					escaped = true
				}
			}
			end := i + 1
			for end < len(data) && (data[end] == ' ' || data[end] == '\t' || data[end] == '\n' || data[end] == '\r') {
				end++
			}
			if end == len(data) || data[end] != ':' {
				continue
			}
			raw := data[start+1 : i]
			var name string
			// Compare decoded names so "a" and "\u0061" count as the same key.
			if escaped || !utf8.Valid(raw) {
				if err := json.Unmarshal(data[start:i+1], &name); err != nil {
					return err
				}
			} else {
				name = string(raw)
			}
			seen := objects[len(objects)-1]
			if _, ok := seen[name]; ok {
				return fmt.Errorf("duplicate JSON object key ending at byte offset %d", i+1)
			}
			seen[name] = struct{}{}
		}
	}
	return nil
}
