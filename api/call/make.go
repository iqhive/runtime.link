package call

import (
	"reflect"
	"strconv"
	"strings"
	"structs"
	"unsafe"

	"runtime.link/api/call/internal/cgo/dyncall"
	"runtime.link/api/call/internal/cgo/dynload"
)

// Promises made about the behaviour of a call.
type Promises uint

const (
	// C ABI call using the platform calling convention.
	C Promises = 0

	// DoesNotReturnGoPointers promises that the function will not return any Go
	// pointers, this allows the runtime to avoid scanning the result for pointers.
	// Can provide a small performance boost if the function is returning an
	// [unsafe.Pointer]. Equivalent to the cgo 'noescape' pragma.
	DoesNotReturnGoPointers Promises = 1 << iota

	// DoesNotCallback promises that the function will not call back into Go code,
	// this enables the runtime to reduce the work required to call into C code.
	// Equivalent to the cgo 'nocallback' pragma. Causes a panic if the function
	// calls back into Go code.
	DoesNotCallback

	// DoesNotBlock promises that the function will not block, this allows the runtime
	// to jump directly into the function. Very high performance but if the promise is
	// broken, all running goroutines may become completely blocked, in the worst case
	// the runtime may deadlock.
	DoesNotBlock
)

// Result that can be returned from a call.
type Result interface {
	~struct{} | ~bool | ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64 | ~uintptr | ~unsafe.Pointer
}

// Returns represents a function that returns a single value of type T.
type Returns[T Result] struct {
	_ structs.HostLayout

	fn    FunctionPointer
	rtype reflect.Kind
	count int
	kinds [10]reflect.Kind

	promises Promises
}

// Make interprets a function pointer with the given behavioural promises and argument types
// so that it can be called from Go.
func Make[T Result](fn FunctionPointer, promises Promises, args ...reflect.Kind) Returns[T] {
	kind := reflect.TypeFor[T]().Kind()
	if reflect.TypeFor[T]() == reflect.TypeFor[struct{}]() {
		kind = reflect.Invalid
	}
	var returns = Returns[T]{
		fn:       fn,
		rtype:    kind,
		count:    len(args),
		promises: promises,
	}
	if len(args) > 10 {
		panic("too many arguments, expected at most 10 got " + strconv.Itoa(len(args)))
	}
	copy(returns.kinds[:], args)
	return returns
}

// Call the function with the given argument pointers, unsafe, the only check, is that the
// number of arguments is correct.
func (fn *Returns[T]) Call(args ...unsafe.Pointer) (result T) {
	if len(args) != fn.count {
		panic("incorrect number of arguments, expected " + strconv.Itoa(fn.count) + " got " + strconv.Itoa(len(args)))
	}
	if fn.fn == 0 {
		panic("function pointer is nil")
	}
	switch {
	case fn.promises&DoesNotBlock != 0:
		jump_call(trampoline, unsafe.Pointer(fn), unsafe.Pointer(&result), unsafe.SliceData(args))
	default:
		dyncall.Slow(unsafe.Pointer(fn), unsafe.Pointer(&result), args...)
	}
	return
}

var trampoline unsafe.Pointer

func init() {
	trampoline = dyncall.GetTrampoline()
}

// Import the given symbol from the given library.
func Import(library string, symbol string) FunctionPointer {
	for split := range strings.SplitSeq(library, ",") {
		lib := dynload.Library(split)
		if lib == nil {
			return 0
		}
		if ptr := dynload.FindSymbol(lib, symbol); ptr != nil {
			return FunctionPointer(ptr)
		}
	}
	return 0
}

// FunctionPointer is an opaque static pointer to a function that can be jumped to.
type FunctionPointer uintptr
