package call_test

import (
	"testing"
	"unsafe"

	"runtime.link/api/call"
	"runtime.link/api/call/callframe"
)

func TestPuts(t *testing.T) {
	puts("Hello, World!\000")
}

func BenchmarkPuts(b *testing.B) {
	for i := 0; i < b.N; i++ {
		puts("Hello, World!\000")
	}
}

var puts_fn = call.Get("libSystem.dylib", "puts")

func puts(s string) {
	var stack [call.StackSize + unsafe.Sizeof(8)]byte
	var local = [...]unsafe.Pointer{
		unsafe.Pointer(unsafe.StringData(s)),
	}
	call.C(stack[:], puts_fn, call.Standard, callframe.New(local[:], callframe.Pointer))
}
