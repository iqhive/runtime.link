//go:build go1.21 && gc

package jit

import "reflect"

func isDirect(a, b reflect.Type) bool {
	if a.NumIn() != b.NumIn() || a.NumOut() != b.NumOut() {
		return false
	}
	for i := 0; i < a.NumIn(); i++ {
		if !sameRegister(a.In(i), b.In(i)) {
			return false
		}
	}
	for i := 0; i < a.NumOut(); i++ {
		if !sameRegister(a.Out(i), b.Out(i)) {
			return false
		}
	}
	return true
}

// sameRegister reports whether a value of either type is passed
// in the same kind of register. A value of any other shape can be
// split across registers in ways that size says nothing about.
func sameRegister(a, b reflect.Type) bool {
	return a.Size() == b.Size() && isRegisterSized(a) && isRegisterSized(b) && isFloating(a) == isFloating(b)
}

func isRegisterSized(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Array, reflect.Struct, reflect.String, reflect.Slice, reflect.Interface, reflect.Complex64, reflect.Complex128:
		return false
	}
	return true
}

func isFloating(t reflect.Type) bool {
	return t.Kind() == reflect.Float32 || t.Kind() == reflect.Float64
}
