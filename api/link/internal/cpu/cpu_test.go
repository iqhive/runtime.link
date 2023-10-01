package cpu

import (
	"testing"
	"unsafe"
)

func TestStuff(t *testing.T) {
	add := func(args ...int64) {
		println(cap(args))
		//fmt.Println(args)
	}

	add(1, 2)

	var evil func(int64, int64, int64) = *(*func(int64, int64, int64))(unsafe.Pointer(&add))
	evil(1, 3, 4)
}
