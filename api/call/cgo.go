package call

import (
	"unsafe"

	"runtime.link/api/call/callframe"
)

// C calls a C function with the given [callframe.Stack], the first pointer
// in the callframe should be a pointer to the result (nil if the function
// does not return anything). The rest of the pointers should be pointers to
// each argument of the function.
func C(fn unsafe.Pointer, promises Promises, stack callframe.Stack) {

}

type Promises uint

const (
	Standard Promises = 0 // no promises

	DoesNotReturnGoPointers Promises = 1 << iota
	DoesNotCallback
)
