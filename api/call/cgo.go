package call

import (
	"unsafe"

	"runtime.link/api/call/callframe"
	"runtime.link/api/call/internal/dyncall"
)

// C calls a C function with the given [callframe.Stack], the first pointer
// in the callframe should be a pointer to the result (nil if the function
// does not return anything). The rest of the pointers should be pointers to
// each argument of the function.
func C(stack []byte, fn unsafe.Pointer, promises Promises, args callframe.Args) {
	dyncall.Standard(stack, fn, args)
}

func Get(library string, name string) unsafe.Pointer {
	lib := dyncall.LoadLibrary(library)
	if lib == nil {
		return nil
	}
	return dyncall.FindSymbol(lib, name)
}

type Promises uint

const (
	Standard Promises = 0 // no promises

	DoesNotReturnGoPointers Promises = 1 << iota
	DoesNotCallback
)
