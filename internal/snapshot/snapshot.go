package snapshot

import (
	"fmt"
	"maps"
	"reflect"
	"time"
)

// Copy isolates mutable storage, including values held in interfaces.
// It handles pointer, map, and slice cycles within one returned copy.
func Copy[T any](value *T) (*T, error) {
	return TransformStrings(value, nil)
}

// TransformStrings copies the configuration and transforms exported string values.
func TransformStrings[T any](value *T, transform func(string) (string, error)) (*T, error) {
	if value == nil {
		return nil, nil
	}
	copy, err := copyValue(reflect.ValueOf(value), make(map[copyVisit]reflect.Value), transform)
	if err != nil {
		return nil, err
	}
	return copy.Interface().(*T), nil
}

type copyVisit struct {
	typeOf reflect.Type
	ptr    uintptr
	length int
}

var timeType = reflect.TypeFor[time.Time]()

func scalar(kind reflect.Kind) bool {
	return kind >= reflect.Bool && kind <= reflect.Complex128 || kind == reflect.String
}

func copyValue(v reflect.Value, seen map[copyVisit]reflect.Value, transform func(string) (string, error)) (reflect.Value, error) {
	t := v.Type()
	if t == timeType {
		// time.Time's shared Location is immutable through its public API.
		return v, nil
	}
	if t.PkgPath() == "sync" || t.PkgPath() == "sync/atomic" {
		return reflect.Value{}, fmt.Errorf("snapshot cannot copy synchronization type %s", t)
	}
	out := reflect.New(t).Elem()
	var visit copyVisit
	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if v.IsNil() {
			return out, nil
		}
		visit = copyVisit{typeOf: t, ptr: v.Pointer()}
		if v.Kind() == reflect.Slice {
			visit.length = v.Len()
		}
		if previous, ok := seen[visit]; ok {
			return previous, nil
		}
	}
	switch v.Kind() {
	case reflect.Pointer:
		out.Set(reflect.New(t.Elem()))
		seen[visit] = out
		elem, err := copyValue(v.Elem(), seen, transform)
		if err != nil {
			return reflect.Value{}, err
		}
		out.Elem().Set(elem)
	case reflect.Interface:
		if !v.IsNil() {
			elem, err := copyValue(v.Elem(), seen, transform)
			if err != nil {
				return reflect.Value{}, err
			}
			out.Set(elem)
		}
	case reflect.Map:
		if entries, ok := v.Interface().(map[string]string); ok && transform == nil {
			cloned := reflect.ValueOf(maps.Clone(entries))
			seen[visit] = cloned
			return cloned, nil
		}
		out.Set(reflect.MakeMapWithSize(t, v.Len()))
		seen[visit] = out
		iter := v.MapRange()
		if scalar(t.Key().Kind()) && scalar(t.Elem().Kind()) {
			key := reflect.New(t.Key()).Elem()
			value := reflect.New(t.Elem()).Elem()
			for iter.Next() {
				key.SetIterKey(iter)
				value.SetIterValue(iter)
				if transform != nil && value.Kind() == reflect.String {
					text, err := transform(value.String())
					if err != nil {
						return reflect.Value{}, err
					}
					value.SetString(text)
				}
				out.SetMapIndex(key, value)
			}
			return out, nil
		}
		for iter.Next() {
			key := iter.Key()
			if transform == nil {
				copied, err := copyValue(key, seen, nil)
				if err != nil {
					return reflect.Value{}, err
				}
				key = copied
			}
			value, err := copyValue(iter.Value(), seen, transform)
			if err != nil {
				return reflect.Value{}, err
			}
			out.SetMapIndex(key, value)
		}
	case reflect.Slice:
		out.Set(reflect.MakeSlice(t, v.Len(), v.Len()))
		seen[visit] = out
		// Scalar elements have no mutable references. Copy their backing storage in bulk.
		kind := t.Elem().Kind()
		if kind >= reflect.Bool && kind <= reflect.Complex128 || kind == reflect.String && transform == nil {
			reflect.Copy(out, v)
			return out, nil
		}
		fallthrough
	case reflect.Array:
		for i := range v.Len() {
			value, err := copyValue(v.Index(i), seen, transform)
			if err != nil {
				return reflect.Value{}, err
			}
			out.Index(i).Set(value)
		}
	case reflect.Struct:
		out.Set(v)
		for i := range v.NumField() {
			field := t.Field(i)
			if !field.IsExported() {
				if !immutableValue(v.Field(i)) {
					return reflect.Value{}, fmt.Errorf("snapshot cannot isolate unexported field %s.%s", t, field.Name)
				}
				continue
			}
			value, err := copyValue(v.Field(i), seen, transform)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("%s.%s: %w", t, field.Name, err)
			}
			out.Field(i).Set(value)
		}
	case reflect.String:
		if transform == nil {
			return v, nil
		}
		text, err := transform(v.String())
		if err != nil {
			return reflect.Value{}, err
		}
		out.SetString(text)
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return reflect.Value{}, fmt.Errorf("snapshot cannot isolate %s", t)
	default:
		out.Set(v)
	}
	return out, nil
}

func immutableValue(v reflect.Value) bool {
	if v.Type() == timeType {
		return true
	}
	if v.Type().PkgPath() == "sync" || v.Type().PkgPath() == "sync/atomic" {
		return false
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface:
		return v.IsNil()
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return false
	case reflect.Array, reflect.Struct:
		n := v.Len
		at := v.Index
		if v.Kind() == reflect.Struct {
			n = v.NumField
			at = v.Field
		}
		for i := range n() {
			if !immutableValue(at(i)) {
				return false
			}
		}
	}
	return true
}
