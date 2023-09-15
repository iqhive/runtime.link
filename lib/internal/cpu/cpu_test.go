package cpu

import (
	"fmt"
	"testing"
	"unsafe"
)

func Generic[T any](a T, b float64) {
	fmt.Println(a, b)
}

func TestStuff(t *testing.T) {
	add := Generic[int64]

	add(1, 2.2)

	var evil func(float64, int64) = *(*func(float64, int64))(unsafe.Pointer(&add))
	evil(1.1, 2)
}
