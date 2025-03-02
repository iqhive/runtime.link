//go:build arm64

package arm64_test

import (
	"fmt"
	"testing"
	"unsafe"

	"runtime.link/cpu/arm64"
)

func TestAbs(t *testing.T) {
	const x = arm64.V[[16]int8](0)
	fn, err := arm64.Compile[func(float64) float64](
		arm64.Abs(x, x),
		arm64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	var i64 = [8]int8{-1, -1, -1, -1, -1, -1, -1, -1}
	f64 := *(*float64)(unsafe.Pointer(&i64))
	val := fn(f64)
	i64 = *(*[8]int8)(unsafe.Pointer(&val))
	if i64 != [8]int8{1, 1, 1, 1, 1, 1, 1, 1} {
		t.Fatal("unexpected value", fmt.Sprintf("%#v", i64))
	}
}

func TestAdd(t *testing.T) {
	const (
		a = arm64.X[int64](0)
		b = arm64.X[int64](1)
		r = arm64.X[int64](0)
	)
	fn, err := arm64.Compile[func(int64, int64) int64](
		arm64.Add(r, a, b, false),
		arm64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fn(1, 2) != 3 {
		t.Fatal("unexpected value", fn(1, 2))
	}
}
