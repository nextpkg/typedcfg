// Package defaults applies default tags to zero-valued struct fields.
package defaults

import (
	"reflect"
	"strconv"
	"strings"
	"time"
)

// SetDefaults fills zero-valued fields from their default tags and recurses into
// nested value structs. Nil pointers require their own default tag to be allocated;
// existing non-nil pointers and collection elements are not traversed.
//
// Supported tag format: `default:"value"`
//
// Examples:
//
//	type Config struct {
//	    Port     int           `default:"8080"`
//	    Host     string        `default:"localhost"`
//	    Timeout  time.Duration `default:"30s"`
//	    Debug    bool          `default:"false"`
//	    Tags     []string      `default:"tag1,tag2,tag3"`
//	}
//
// Non-struct inputs are ignored; invalid default values return conversion errors.
func SetDefaults(ptr any) error {
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
			if err := SetDefaults(field.Addr().Interface()); err != nil {
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

		if err := setFieldValue(field, defaultValue); err != nil {
			return err
		}
	}

	return nil
}

// setFieldValue parses a default tag into a supported field type.
func setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			// Handle time.Duration
			duration, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			field.SetInt(int64(duration))
		} else {
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			field.SetInt(intVal)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(uintVal)

	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
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

	case reflect.Slice:
		// Handle string slices (comma-separated values)
		if field.Type().Elem().Kind() == reflect.String {
			if value != "" {
				sliceVal := reflect.MakeSlice(field.Type(), 0, 0)
				for _, item := range splitAndTrim(value, ",") {
					sliceVal = reflect.Append(sliceVal, reflect.ValueOf(item))
				}
				field.Set(sliceVal)
			}
		}

	case reflect.Struct:
		// Recursively handle nested structs
		if field.CanAddr() {
			return SetDefaults(field.Addr().Interface())
		}

	case reflect.Pointer:
		// Handle pointer fields
		if field.IsNil() {
			// Create new instance
			newVal := reflect.New(field.Type().Elem())
			field.Set(newVal)
		}
		// Set the value for the pointed-to element
		if field.Elem().Kind() == reflect.Struct {
			return SetDefaults(field.Interface())
		} else {
			// For non-struct pointers, set the value directly
			return setFieldValue(field.Elem(), value)
		}
	}

	return nil
}

// splitAndTrim splits a tag and discards whitespace-only elements.
func splitAndTrim(s, delimiter string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	for part := range strings.SplitSeq(s, delimiter) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}
