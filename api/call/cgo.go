package call

import (
	"unsafe"

	"runtime.link/api/call/callframe"
	"runtime.link/api/call/internal/cgo/dyncall"
	"runtime.link/api/call/internal/cgo/dynload"
)

// C calls a C function with the given [callframe.Stack], the first pointer
// in the callframe should be a pointer to the result (nil if the function
// does not return anything). The rest of the pointers should be pointers to
// each argument of the function.
func C(fn unsafe.Pointer, promises Promises, ptrs ...unsafe.Pointer) {
	var codes = [...]callframe.Code{
		callframe.Float64,
		callframe.Float64,
	}
	jump_call(trampoline, fn, unsafe.Pointer(&ptrs[0]), unsafe.Pointer(&codes[0]))
}

//go:noescape
func jump_call(trampoline, fn, callframe, codes unsafe.Pointer)

var trampoline unsafe.Pointer

func init() {
	trampoline = dyncall.GetTrampoline()
}

func Get(library string, name string) unsafe.Pointer {
	lib := dynload.Library(library)
	if lib == nil {
		return nil
	}
	return dynload.FindSymbol(lib, name)
}

type Promises uint

const (
	Standard Promises = 0 // no promises

	DoesNotReturnGoPointers Promises = 1 << iota
	DoesNotCallback
)
