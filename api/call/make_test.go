package call_test

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"unsafe"

	"runtime.link/api/call"
)

func TestPuts(t *testing.T) {
	puts("Hello, World!\000")
	fmt.Println("sin", sin(0.5))
}

func BenchmarkPuts(b *testing.B) {
	var sum float64
	for i := 0; i < b.N; i++ {
		sum += sin(0.5)
	}
	_ = sum
}

func BenchmarkPrintln(b *testing.B) {
	var sum float64
	for i := 0; i < b.N; i++ {
		sum += math.Sin(0.5)
	}
	_ = sum
}

var c_puts = call.Make[struct{}](call.Import("libc.so.6,libSystem.dylib", "puts"), call.C, reflect.Pointer)

func puts(s string) {
	c_puts.Call(unsafe.Pointer(unsafe.StringData(s + "\000")))
}

var c_sin = call.Make[float64](call.Import("libm.so.6,libSystem.dylib", "sin"), call.C|call.DoesNotBlock, reflect.Float64)

func sin(x float64) (r float64) {
	return c_sin.Call(unsafe.Pointer(&x))
}
