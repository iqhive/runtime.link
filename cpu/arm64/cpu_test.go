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

func TestSub(t *testing.T) {
	const (
		a = arm64.X[int64](0)
		b = arm64.X[int64](1)
		r = arm64.X[int64](0)
	)
	fn, err := arm64.Compile[func(int64, int64) int64](
		arm64.Sub(r, a, b, false),
		arm64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fn(5, 2) != 3 {
		t.Fatal("unexpected value", fn(5, 2))
	}
}

func TestBitwiseAnd(t *testing.T) {
	const (
		a = arm64.X[int64](0)
		b = arm64.X[int64](1)
		r = arm64.X[int64](0)
	)
	fn, err := arm64.Compile[func(int64, int64) int64](
		arm64.BitwiseAnd(r, a, b, false),
		arm64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fn(3, 1) != 1 {
		t.Fatal("unexpected value", fn(3, 1))
	}
}

func TestBitwiseOr(t *testing.T) {
	const (
		a = arm64.X[int64](0)
		b = arm64.X[int64](1)
		r = arm64.X[int64](0)
	)
	fn, err := arm64.Compile[func(int64, int64) int64](
		arm64.BitwiseOr(r, a, b),
		arm64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fn(2, 1) != 3 {
		t.Fatal("unexpected value", fn(2, 1))
	}
}

func TestBitwiseXor(t *testing.T) {
	const (
		a = arm64.X[int64](0)
		b = arm64.X[int64](1)
		r = arm64.X[int64](0)
	)
	fn, err := arm64.Compile[func(int64, int64) int64](
		arm64.BitwiseXor(r, a, b),
		arm64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fn(3, 1) != 2 {
		t.Fatal("unexpected value", fn(3, 1))
	}
}

func TestMoveWideImmediate(t *testing.T) {
	const (
		r = arm64.X[uint64](0)
	)
	fn, err := arm64.Compile[func() uint64](
		arm64.MoveWide(r, 0x1234, 0),
		arm64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fn() != 0x1234 {
		t.Fatal("unexpected value", fn())
	}
}

func TestCompare(t *testing.T) {
	const (
		a = arm64.X[int64](0)
		b = arm64.X[int64](1)
		r = arm64.X[int64](0)
	)
	fn, err := arm64.Compile[func(int64, int64) int64](
		arm64.Compare(a, b),
		arm64.Add(r, a, b, false), // Just to have a result to return
		arm64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	// Just test that it compiles and runs without error
	fn(5, 2)
}
