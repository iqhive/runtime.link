package call

import (
	"reflect"
	"unsafe"
)

type Value struct {
	value complex128
	point unsafe.Pointer
	rtype reflect.Type
}

func (v Value) Int64() int64 {
	if v.rtype.Kind() != reflect.Int64 {
		panic("not an int64")
	}
	return *(*int64)(unsafe.Pointer(&v.value))
}
