package defaulttags

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Apply fills zero fields and optionally expands scalar defaults before parsing.
func Apply(ptr any, transform func(string) (string, error)) error {
	if ptr == nil {
		return nil
	}

	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return nil
	}

	v = v.Elem()
	t := v.Type()

	for i := range v.NumField() {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Handle nested structs recursively
		if field.Kind() == reflect.Struct {
			if err := Apply(field.Addr().Interface(), transform); err != nil {
				return err
			}
			continue
		}

		// Get default tag value
		defaultValue, ok := fieldType.Tag.Lookup("default")
		if !ok {
			continue
		}

		// Only set default if field is zero value
		if !field.IsZero() {
			continue
		}

		if err := assign(field, defaultValue, transform); err != nil {
			return fmt.Errorf("default for %s: %w", fieldType.Name, err)
		}
	}
	return nil
}

func assign(field reflect.Value, value string, transform func(string) (string, error)) error {
	for field.Kind() == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}
	switch field.Kind() {
	case reflect.Struct:
		return Apply(field.Addr().Interface(), transform)
	case reflect.String:
		field.SetString(value)
	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String && value != "" {
			items := splitAndTrim(value, ",")
			slice := reflect.MakeSlice(field.Type(), len(items), len(items))
			for i, item := range items {
				slice.Index(i).SetString(item)
			}
			field.Set(slice)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Bool, reflect.Float32, reflect.Float64:
		raw := value
		if transform != nil {
			var err error
			raw, err = transform(value)
			if err != nil {
				return err
			}
		}
		err := parseScalar(field, raw)
		if err != nil && raw != value {
			return fmt.Errorf("invalid expanded %s", field.Type())
		}
		return err
	}
	return nil
}

func parseScalar(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			// Handle time.Duration
			duration, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			field.SetInt(int64(duration))
		} else {
			intVal, err := strconv.ParseInt(value, 10, field.Type().Bits())
			if err != nil {
				return err
			}
			field.SetInt(intVal)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(uintVal)

	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(floatVal)

	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(boolVal)

	}
	return nil
}

// splitAndTrim splits a tag and discards whitespace-only elements.
func splitAndTrim(s, delimiter string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0, strings.Count(s, delimiter)+1)
	for part := range strings.SplitSeq(s, delimiter) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}
