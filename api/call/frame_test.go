package call_test

import (
	"fmt"
	"testing"
	"unsafe"

	"runtime.link/api/call"
)

func TestFrame(t *testing.T) {
	var frame = call.New()

	ptr := call.Arg(frame, 100)

	fmt.Println(*(*int)(unsafe.Pointer(ptr.Uintptr())))
}

var TestEscape = func(a, b, c *int, ret *int) { *ret = 22 }

func BenchmarkFrame(b *testing.B) {
	var frame = call.New()
	b.ResetTimer()
	frame.Free()

	for i := 0; i < b.N; i++ {
		frame = call.New()
		ptr1 := call.Arg(frame, 100)
		ptr2 := call.Arg(frame, 200)
		ptr3 := call.Arg(frame, 300)
		ret := call.Ret[int](frame)
		TestEscape((*int)(unsafe.Pointer(ptr1.Uintptr())), (*int)(unsafe.Pointer(ptr2.Uintptr())), (*int)(unsafe.Pointer(ptr3.Uintptr())), (*int)(unsafe.Pointer(ret.Uintptr())))
		if ret.Get() != 22 {
			b.Fatal("ret != 22")
		}
		frame.Free()
	}
}

func BenchmarkEscape(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var arg = 100
		var arg2 = 200
		var arg3 = 200
		var ret int
		TestEscape(&arg, &arg2, &arg3, &ret)
		if ret != 22 {
			b.Fatal("ret != 22")
		}
	}
}
