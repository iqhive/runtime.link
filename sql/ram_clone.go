package sql

import (
	"net/netip"
	"reflect"
	"time"
)

// copiesShareMemory reports whether a copy of a value of the type shares memory
// with it that [cloneRAMValue] would give the copy for itself. Most rows are
// plain values that do not, and need not be walked to be copied.
func copiesShareMemory(rtype reflect.Type) bool {
	switch rtype.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface:
		return true
	case reflect.Array:
		return copiesShareMemory(rtype.Elem())
	case reflect.Struct:
		// These immutable values use shared identity for their location or zone.
		switch rtype {
		case reflect.TypeFor[time.Time](), reflect.TypeFor[netip.Addr]():
			return false
		}
		for i := range rtype.NumField() {
			if field := rtype.Field(i); copiesShareMemory(field.Type) {
				return true
			}
		}
	}
	return false
}

// cloneRAMValue keeps patches and callers from changing a row version through
// shared references, including those inside private fields. Identity-bearing
// channels, functions and map keys retain their identity.
func cloneRAMValue[V any](value V) V {
	result := reflect.New(reflect.TypeOf((*V)(nil)).Elem()).Elem()
	result.Set(cloneRAMReflect(reflect.ValueOf(&value).Elem(), make(map[ramCloneRef]reflect.Value)))
	return *result.Addr().Interface().(*V)
}

type ramCloneRef struct {
	typ      reflect.Type
	ptr      any
	len, cap int
}

func cloneRAMReflect(value reflect.Value, seen map[ramCloneRef]reflect.Value) reflect.Value {
	if !copiesShareMemory(value.Type()) {
		return value
	}
	result := reflect.New(value.Type()).Elem()
	result.Set(value)
	switch value.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if value.IsNil() {
			return result
		}
		ref := ramCloneRef{typ: value.Type(), ptr: value.UnsafePointer()}
		if value.Kind() == reflect.Slice {
			ref.len, ref.cap = value.Len(), value.Cap()
		}
		if prior, ok := seen[ref]; ok {
			return prior
		}
		switch value.Kind() {
		case reflect.Pointer:
			result.Set(reflect.New(value.Type().Elem()))
			seen[ref] = result
			result.Elem().Set(cloneRAMReflect(value.Elem(), seen))
		case reflect.Map:
			result.Set(reflect.MakeMapWithSize(value.Type(), value.Len()))
			seen[ref] = result
			iter := value.MapRange()
			for iter.Next() {
				result.SetMapIndex(iter.Key(), cloneRAMReflect(iter.Value(), seen))
			}
		case reflect.Slice:
			result.Set(reflect.MakeSlice(value.Type(), value.Len(), value.Cap()))
			seen[ref] = result
			source, target := value.Slice(0, value.Cap()), result.Slice(0, result.Cap())
			for i := 0; i < source.Len(); i++ {
				target.Index(i).Set(cloneRAMReflect(source.Index(i), seen))
			}
		}
	case reflect.Interface:
		if !value.IsNil() {
			result.Set(cloneRAMReflect(value.Elem(), seen))
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := result.Field(i)
			if !copiesShareMemory(field.Type()) {
				continue
			}
			// result owns an addressable copy. Access private fields on that
			// copy so cloning a row never edits the caller's value.
			if !field.CanSet() {
				field = reflect.NewAt(field.Type(), field.Addr().UnsafePointer()).Elem()
			}
			field.Set(cloneRAMReflect(field, seen))
		}
	case reflect.Array:
		for i := 0; i < value.Len(); i++ {
			result.Index(i).Set(cloneRAMReflect(value.Index(i), seen))
		}
	}
	return result
}
