package cpu

import (
	"testing"
	"unsafe"
)

func TestStuff(t *testing.T) {
	add := func(a int64, b float64) {
		println(a, b)
	}

	add(1, 2.2)

	var evil func(float64, int64) = *(*func(float64, int64))(unsafe.Pointer(&add))
	evil(1.1, 2)
}
