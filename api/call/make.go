package call

import (
	"reflect"
	"strconv"
	"strings"
	"structs"
	"sync"
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
	~bool | ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64 | ~uintptr | ~unsafe.Pointer
}

// UnsafeReturns represents a function that returns a single value of type T.
type UnsafeReturns[T Result] struct {
	Unsafe
}

type Unsafe struct {
	_ structs.HostLayout

	fn    FunctionPointer
	rtype reflect.Kind
	count int
	kinds [10]reflect.Kind

	promises Promises
}

func Void(fn FunctionPointer, promises Promises, args ...reflect.Kind) Unsafe {
	var returns = Unsafe{
		fn:       fn,
		rtype:    reflect.Invalid,
		count:    len(args),
		promises: promises,
	}
	if len(args) > 10 {
		panic("too many arguments, expected at most 10 got " + strconv.Itoa(len(args)))
	}
	copy(returns.kinds[:], args)
	return returns
}

// Returns interprets a function pointer with the given behavioural promises and argument types
// so that it can be called from Go.
func Returns[T Result](fn FunctionPointer, promises Promises, args ...reflect.Kind) UnsafeReturns[T] {
	var returns = UnsafeReturns[T]{
		Unsafe: Unsafe{
			fn:       fn,
			rtype:    reflect.TypeFor[T]().Kind(),
			count:    len(args),
			promises: promises,
		},
	}
	if len(args) > 10 {
		panic("too many arguments, expected at most 10 got " + strconv.Itoa(len(args)))
	}
	copy(returns.kinds[:], args)
	return returns
}

// Call the function with the given argument pointers, unsafe, the only check, is that the
// number of arguments is correct.
func (fn *UnsafeReturns[T]) Call(args ...unsafe.Pointer) (result T) {
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

// Call the function with the given argument pointers, unsafe, the only check, is that the
// number of arguments is correct.
func (fn *Unsafe) Call(args ...unsafe.Pointer) {
	if len(args) != fn.count {
		panic("incorrect number of arguments, expected " + strconv.Itoa(fn.count) + " got " + strconv.Itoa(len(args)))
	}
	if fn.fn == 0 {
		panic("function pointer is nil")
	}
	switch {
	case fn.promises&DoesNotBlock != 0:
		jump_call(trampoline, unsafe.Pointer(fn), nil, unsafe.SliceData(args))
	default:
		dyncall.Slow(unsafe.Pointer(fn), nil, args...)
	}
	return
}

// Make a callable function of type T
func Make[T any](fn FunctionPointer, promises Promises) T {
	return MakeFunc(reflect.TypeFor[T](), fn, promises).Interface().(T)
}

// MakeFunc returns a function of the given type that can be called.
func MakeFunc(typ reflect.Type, fn FunctionPointer, promises Promises) reflect.Value {
	if typ.Kind() != reflect.Func {
		panic("expected a function type")
	}
	if typ.NumOut() > 1 {
		panic("expected at most one return value")
	}
	var returns reflect.Kind
	if typ.NumOut() == 1 {
		returns = typ.Out(0).Kind()
	}
	var args = make([]reflect.Kind, typ.NumIn())
	for i := range args {
		args[i] = typ.In(i).Kind()
	}
	var caller = Unsafe{
		fn:       fn,
		rtype:    returns,
		count:    len(args),
		promises: promises,
	}
	copy(caller.kinds[:], args)
	if fn == 0 {
		return reflect.MakeFunc(typ, func(args []reflect.Value) []reflect.Value {
			panic("function pointer is nil")
		})
	}
	switch {
	case promises&DoesNotBlock != 0:
		if returns != reflect.Invalid {
			return reflect.MakeFunc(typ, func(args []reflect.Value) []reflect.Value {
				var ptrs = make([]unsafe.Pointer, len(args))
				for i, arg := range args {
					ptrs[i] = unsafe.Pointer(arg.UnsafeAddr())
				}
				var result = reflect.New(typ.Out(0))
				jump_call(trampoline, unsafe.Pointer(&caller), result.UnsafePointer(), unsafe.SliceData(ptrs))
				return []reflect.Value{result.Elem()}
			})
		} else {
			return reflect.MakeFunc(typ, func(args []reflect.Value) []reflect.Value {
				var ptrs = make([]unsafe.Pointer, len(args))
				for i, arg := range args {
					ptrs[i] = unsafe.Pointer(arg.UnsafeAddr())
				}
				jump_call(trampoline, unsafe.Pointer(&caller), nil, unsafe.SliceData(ptrs))
				return nil
			})
		}
	default:
		if returns != reflect.Invalid {
			return reflect.MakeFunc(typ, func(args []reflect.Value) []reflect.Value {
				var ptrs = make([]unsafe.Pointer, len(args))
				for i, arg := range args {
					ptr := reflect.New(typ.In(i))
					ptr.Elem().Set(arg)
					ptrs[i] = ptr.UnsafePointer()
				}
				var result = reflect.New(typ.Out(0))
				dyncall.Slow(unsafe.Pointer(&caller), result.UnsafePointer(), ptrs...)
				return []reflect.Value{result.Elem()}
			})
		} else {
			return reflect.MakeFunc(typ, func(args []reflect.Value) []reflect.Value {
				var ptrs = make([]unsafe.Pointer, len(args))
				for i, arg := range args {
					ptr := reflect.New(typ.In(i))
					ptr.Elem().Set(arg)
					ptrs[i] = ptr.UnsafePointer()
				}
				dyncall.Slow(unsafe.Pointer(&caller), nil, ptrs...)
				return nil
			})
		}
	}
}

var trampoline uintptr

func init() {
	trampoline = dyncall.GetTrampoline()
}

var loaded_library = make(map[string]dynload.LibraryPointer)
var mutex sync.Mutex

// Import the given symbol from the given library.
func Import(library string, symbol string) FunctionPointer {
	for split := range strings.SplitSeq(library, ",") {
		mutex.Lock()
		lib, ok := loaded_library[split]
		if !ok {
			lib = dynload.Library(split)
			loaded_library[split] = lib
		}
		mutex.Unlock()
		if lib == nil {
			continue
		}
		if ptr := dynload.FindSymbol(lib, symbol); ptr != nil {
			return FunctionPointer(ptr)
		}
	}
	return 0
}

// FunctionPointer is an opaque static pointer to a function that can be jumped to.
type FunctionPointer uintptr
