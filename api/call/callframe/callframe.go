// Package callframe provides an unsafe stack-based callframe representation for calling foreign functions at an assembly level.
package callframe

import (
	"unsafe"
)

// Stack represents a set of arguments to be passed on the stack to a
// foreign function.
type Stack struct {
	local []unsafe.Pointer
	codes []Code
}

// New creates a new Stack, pass a pointer to each argument and then the
// corresponding kind of each argument. You should allocate the slice of
// pointers on the stack using an array.
func New(local []unsafe.Pointer, codes ...Code) Stack {
	return Stack{
		local: local,
		codes: codes,
	}
}

// Code used to classify the kind and size of the arguments to
// be passed to the foreign function.
type Code uint8

const (
	Ignored Code = iota
	Binary1      // bool, int8, uint8
	Binary2      // int16, uint16
	Binary4      // int32, uint32
	Binary8      // int64, uint64
	Float32      // float32
	Float64      // float64
	Pointer      // uintptr, unsafe.Pointer, *T
	Repeats      // [N]T
	Offsets      // struct
)
