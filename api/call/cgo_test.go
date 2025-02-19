package call_test

import (
	"fmt"
	"math"
	"testing"
	"unsafe"

	"runtime.link/api/call"
)

func TestPuts(t *testing.T) {
	puts("Hello, World!\000")
	fmt.Println(sin(0.5))
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

var puts_fn = call.Get("libSystem.dylib", "puts")

func puts(s string) {
	/*var stack [call.StackSize + unsafe.Sizeof(8)]byte
	var local = [...]unsafe.Pointer{
		nil,
		unsafe.Pointer(unsafe.StringData(s)),
	}
	call.C[struct{}](puts_fn, call.Standard, callframe.New(local[:], callframe.Ignored, callframe.Pointer))*/
}

var libSystem_sin = call.New[float64]("libSystem.dylib", "sin", call.DoesNotCallback|call.DoesNotReturnGoPointers, call.Float64)

func sin(x float64) (r float64) {
	return call.C(libSystem_sin, unsafe.Pointer(&x))
}
