//go:build cgo

package dyncall

/*
#include <dyncall_layout.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// Frame is what a function on arm64 would have been called with.
type Frame struct {
	X     [8]uint64 // x0 to x7.
	D     [8]uint64 // the bits of d0 to d7, a float being in the low half.
	Stack []byte    // the arguments on the stack, with the first at the start.
}

// LayoutARM64 returns where a call on arm64 would put the arguments that have
// been pushed, on Apple's platforms or else on the others, and calls nothing.
// It is here for the tests, which can then check the convention of a platform
// that they are not running on, and Apple's is otherwise only run by a Mac.
// The arguments go through everything that a call puts them through, apart
// from the last step of loading them and calling.
func (vm *VM) LayoutARM64(apple bool) (Frame, error) {
	var frame C.GoFrame
	var asApple C.int
	if apple {
		asApple = 1
	}
	C.goLayoutARM64(asApple, unsafe.SliceData(vm.buf), C.int(len(vm.buf)), vm.fixed, &frame)
	if int(frame.size) > len(frame.stack) {
		return Frame{}, fmt.Errorf("dyncall: %d bytes of arguments on the stack is more than there is room to record", frame.size)
	}
	var result Frame
	for i := range result.X {
		result.X[i] = uint64(frame.x[i])
		result.D[i] = *(*uint64)(unsafe.Pointer(&frame.d[i]))
	}
	result.Stack = C.GoBytes(unsafe.Pointer(&frame.stack[0]), C.int(frame.size))
	return result, nil
}

// ReadAppleARM64 returns the arguments that a function called back by C on
// Apple's arm64 reads from frame, when they have the types of the signature:
// an integer widened as its type is, and the bits of a float or a double. Like
// [VM.LayoutARM64] it is here for the tests, and runs the code that a callback
// on a Mac reads its arguments with, on a frame that no call made.
func (frame Frame) ReadAppleARM64(sig []rune) ([]uint64, error) {
	var from C.GoFrame
	if len(frame.Stack) > len(from.stack) {
		return nil, fmt.Errorf("dyncall: %d bytes of arguments on the stack is more than there is room for", len(frame.Stack))
	}
	for i := range frame.X {
		from.x[i] = C.ulonglong(frame.X[i])
		*(*uint64)(unsafe.Pointer(&from.d[i])) = frame.D[i]
	}
	for i, b := range frame.Stack {
		from.stack[i] = C.uchar(b)
	}
	from.size = C.size_t(len(frame.Stack))
	if len(sig) == 0 {
		return nil, nil
	}
	types := make([]C.DCsigchar, len(sig))
	for i, char := range sig {
		types[i] = C.DCsigchar(char)
	}
	values := make([]C.ulonglong, len(sig))
	if C.goAppleReadARM64(&from, unsafe.SliceData(types), C.int(len(types)), unsafe.SliceData(values)) == 0 {
		return nil, fmt.Errorf("dyncall: unable to read the arguments %q from %d bytes of stack", string(sig), len(frame.Stack))
	}
	result := make([]uint64, len(values))
	for i, value := range values {
		result[i] = uint64(value)
	}
	return result, nil
}
