//go:build go1.21 && gc

package jit

import "reflect"

func isDirect(a, b reflect.Type) bool {
	if a.NumIn() != b.NumIn() || a.NumOut() != b.NumOut() {
		return false
	}
	for i := 0; i < a.NumIn(); i++ {
		if a.Size() != b.Size() {
			return false
		}
	}
	// FIXME classify each register.
	return true
}
