package jit

import (
	"reflect"
	"unsafe"

	"runtime.link/xyz"
)

type gprValue = struct{ gpr }

type fprValue = struct{ fpr }

type ptrValue[T any] struct {
	isptr [0]*T
	value unsafe.Pointer
}

// JIT equivalents to all Go types.
type (
	Bool      struct{ gpr }
	Int       struct{ gpr }
	Int8      struct{ gpr }
	Int16     struct{ gpr }
	Int32     struct{ gpr }
	Int64     struct{ gpr }
	Uint      struct{ gpr }
	Uint8     struct{ gpr }
	Uint16    struct{ gpr }
	Uint32    struct{ gpr }
	Uint64    struct{ gpr }
	Uintptr   struct{ gpr }
	Float32   struct{ fpr }
	Float64   struct{ fpr }
	Complex64 struct {
		real fpr
		imag fpr
	}
	Complex128 struct {
		real fpr
		imag fpr
	}
	Chan[T any]      ptrValue[chan T]
	Func[T any]      ptrValue[Func[T]]
	Interface[T any] struct {
		rtype ptrValue[reflect.Type]
		value ptrValue[unsafe.Pointer]
	}
	Map[K comparable, V any] ptrValue[map[K]V]
	Pointer[T any]           ptrValue[T]
	Slice[T any]             struct {
		ptr ptrValue[T]
		len gpr
		cap gpr
	}
	String struct {
		ptr ptrValue[byte]
		len gpr
	}
	UnsafePointer ptrValue[unsafe.Pointer]
)

// Value represents an underlying Go value.
type Value struct {
	direct reflect.Value // when sourced from [reflect.MakeFunc]
}

type Location xyz.Switch[int, struct {
}]
