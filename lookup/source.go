// Package lookup reads explicitly named scalar settings from a key-value lookup.
package lookup

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/nextpkg/typedcfg"
)

// Source builds a JSON source for the exported, explicitly named JSON fields
// of a flat struct. Missing keys are omitted. Invalid values reject the read.
func Source[T any](lookup func(string) (string, bool)) (typedcfg.Source, error) {
	typ := reflect.TypeFor[T]()
	if typ.Kind() != reflect.Struct || lookup == nil {
		return nil, fmt.Errorf("lookup requires a struct and a lookup function")
	}
	type field struct {
		name  string
		parse func(string) (any, error)
	}
	var fields []field
	seen := make(map[string]bool)
	for f := range typ.Fields() {
		name, options, _ := strings.Cut(f.Tag.Get("json"), ",")
		if !f.IsExported() || name == "" || name == "-" {
			continue
		}
		parse := scalarParser(f.Type)
		if parse == nil || seen[name] || (options != "" && options != "omitempty") {
			return nil, fmt.Errorf("unsupported or duplicate lookup field %q", f.Name)
		}
		seen[name] = true
		fields = append(fields, field{name, parse})
	}
	return func(ctx context.Context) ([]byte, error) {
		values := make(map[string]any, len(fields))
		for _, f := range fields {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			raw, ok := lookup(f.name)
			if !ok {
				continue
			}
			value, err := f.parse(raw)
			if err != nil {
				return nil, fmt.Errorf("invalid value for lookup key %q", f.name)
			}
			values[f.name] = value
		}
		return json.Marshal(values)
	}, nil
}

func scalarParser(t reflect.Type) func(string) (any, error) {
	if t == reflect.TypeFor[time.Duration]() {
		return func(s string) (any, error) { return time.ParseDuration(s) }
	}
	switch t.Kind() {
	case reflect.String:
		return func(s string) (any, error) { return s, nil }
	case reflect.Bool:
		return func(s string) (any, error) { return strconv.ParseBool(s) }
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return func(s string) (any, error) { return strconv.ParseInt(s, 10, t.Bits()) }
	case reflect.Float32, reflect.Float64:
		return func(s string) (any, error) {
			value, err := strconv.ParseFloat(s, t.Bits())
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, fmt.Errorf("expected finite number")
			}
			return value, err
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return func(s string) (any, error) { return strconv.ParseUint(s, 10, t.Bits()) }
	}
	return nil
}
