//go:build !go1.21 || !gc

package jit

import "reflect"

func isDirect(a, b reflect.Type) bool { return false }
