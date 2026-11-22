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
	result, _, err := TransformStrings(value, nil)
	return result, err
}

// TransformStrings reports whether the copy is isolated from its input.
// Expansion leaves map keys literal, including any pointers reachable from a key.
func TransformStrings[T any](value *T, transform func(string) (string, error)) (*T, bool, error) {
	if value == nil {
		return nil, true, nil
	}
	c := copier{seen: make(map[copyVisit]reflect.Value)}
	result, err := c.copyValue(reflect.ValueOf(value), transform)
	if err != nil {
		return nil, false, err
	}
	return result.Interface().(*T), !c.borrowedKeys, nil
}

type copier struct {
	seen         map[copyVisit]reflect.Value
	borrowedKeys bool
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

func (c *copier) copyValue(v reflect.Value, transform func(string) (string, error)) (reflect.Value, error) {
	t := v.Type()
	if t == timeType {
		// time.Time's shared Location is immutable through its public API.
		return v, nil
	}
	if t.PkgPath() == "sync" || t.PkgPath() == "sync/atomic" {
		return reflect.Value{}, fmt.Errorf("snapshot cannot copy synchronization type %s", t)
	}
	if scalar(v.Kind()) && (transform == nil || v.Kind() != reflect.String) {
		return v, nil
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
			// Slices starting at the same element can have different lengths.
			visit.length = v.Len()
		}
		if previous, ok := c.seen[visit]; ok {
			return previous, nil
		}
	}
	switch v.Kind() {
	case reflect.Pointer:
		out.Set(reflect.New(t.Elem()))
		// Register the destination before descending so a cycle points back to it.
		c.seen[visit] = out
		elem, err := c.copyValue(v.Elem(), transform)
		if err != nil {
			return reflect.Value{}, err
		}
		out.Elem().Set(elem)
	case reflect.Interface:
		if !v.IsNil() {
			elem, err := c.copyValue(v.Elem(), transform)
			if err != nil {
				return reflect.Value{}, err
			}
			out.Set(elem)
		}
	case reflect.Map:
		if entries, ok := v.Interface().(map[string]string); ok && transform == nil {
			cloned := reflect.ValueOf(maps.Clone(entries))
			c.seen[visit] = cloned
			return cloned, nil
		}
		out.Set(reflect.MakeMapWithSize(t, v.Len()))
		c.seen[visit] = out
		if transform != nil && !scalar(t.Key().Kind()) {
			// Expansion preserves keys. A key containing a pointer can still
			// reach the decoder's data, so publication requires a second copy.
			c.borrowedKeys = true
		}
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
				copied, err := c.copyValue(key, nil)
				if err != nil {
					return reflect.Value{}, err
				}
				key = copied
			}
			value, err := c.copyValue(iter.Value(), transform)
			if err != nil {
				return reflect.Value{}, err
			}
			out.SetMapIndex(key, value)
		}
	case reflect.Slice:
		out.Set(reflect.MakeSlice(t, v.Len(), v.Len()))
		c.seen[visit] = out
		// Scalar elements have no mutable references. Copy their backing storage in bulk.
		kind := t.Elem().Kind()
		if kind >= reflect.Bool && kind <= reflect.Complex128 || kind == reflect.String && transform == nil {
			reflect.Copy(out, v)
			return out, nil
		}
		fallthrough
	case reflect.Array:
		for i := range v.Len() {
			value, err := c.copyValue(v.Index(i), transform)
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
			value, err := c.copyValue(v.Field(i), transform)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("%s.%s: %w", t, field.Name, err)
			}
			out.Field(i).Set(value)
		}
	case reflect.String:
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
