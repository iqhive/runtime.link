//go:build cgo

package dyncall

/*
#include <assert.h>
#include <dyncall.h>
#include <dyncall_callback.h>
#include <stdint.h>
#include <stdlib.h>

extern DCsigchar bridge_callback(DCCallback*, DCArgs*, DCValue*, uintptr_t);

DCCallback *goNewCallback(const DCsigchar * signature, uintptr_t userdata) {
	return dcbNewCallback(signature, (DCCallbackHandler*)bridge_callback, (void*)userdata);
}

// A handle is a small integer, which the Go runtime rejects if it ever
// finds one in a pointer slot, so it must not cross the boundary as void*.
uintptr_t goCallbackHandle(DCCallback *callback) {
	return (uintptr_t)dcbGetUserData(callback);
}

#include <dyncall_go.h>

double goCallDouble(DCCallVM *vm, DCpointer funcptr, GoArg *arg, int argc, int fixed) {
	goArgs(vm, arg, argc, fixed);
	return dcCallDouble(vm, funcptr);
}

*/
import "C"
import (
	"runtime/cgo"
	"unsafe"
)

type Callback C.DCCallback

type CallbackHandler func(*Callback, *Args, unsafe.Pointer) rune

// NewCallback returns a C function pointer that calls handler, or nil
// if the memory for it could not be allocated.
func NewCallback(sig Signature, handler CallbackHandler) *Callback {
	s := C.CString(string(sig.Args) + ")" + string(sig.Returns))
	defer C.free(unsafe.Pointer(s))
	handle := cgo.NewHandle(handler)
	callback := C.goNewCallback((*C.DCsigchar)(s), C.uintptr_t(handle))
	if callback == nil {
		handle.Delete()
	}
	return (*Callback)(callback)
}

func (callback *Callback) Free() {
	handle := cgo.Handle(C.goCallbackHandle((*C.DCCallback)(callback)))
	C.dcbFreeCallback((*C.DCCallback)(callback))
	handle.Delete()
}

// maxStringLen is how far a C string is followed for the NUL that ends it,
// which the C ABI requires a char * to have. A library that hands back a
// pointer to memory that never ends is followed at most this far, rather
// than read and copied without a limit.
const maxStringLen = 1 << 24 // 16 MiB, more than any string a C API returns.

// GoString copies the NUL-terminated string at ptr. nil is the empty string.
// C.GoString would follow the string without a bound of its own, so the
// length is found here first, and a string that ends in no NUL within
// [maxStringLen] is refused rather than trusted to end somewhere.
func GoString(ptr unsafe.Pointer) string {
	if ptr == nil {
		return ""
	}
	for length := 0; ; length++ {
		if *(*byte)(unsafe.Add(ptr, uintptr(length))) == 0 {
			return string(unsafe.Slice((*byte)(ptr), length))
		}
		if length == maxStringLen {
			panic("dyncall: a char * is not NUL-terminated within 16 MiB, which the C ABI requires")
		}
	}
}

// CString copies s, with a NUL after it, into memory from malloc, which C may free.
func CString(s string) unsafe.Pointer {
	return unsafe.Pointer(C.CString(s))
}

// Free releases memory from malloc.
func Free(ptr unsafe.Pointer) {
	C.free(ptr)
}

type VM struct {
	ptr   *C.DCCallVM
	buf   []C.GoArg
	fixed C.int // the number of named arguments of a variadic function, negative for any other.
}

func NewVM(size int) *VM {
	return &VM{
		ptr:   C.dcNewCallVM(C.size_t(size)),
		buf:   make([]C.GoArg, 0),
		fixed: -1,
	}
}

func (vm *VM) Reset() {
	vm.buf = vm.buf[:0]
	vm.fixed = -1
}

// Variadic says that the function about to be called has variable arguments,
// which are those pushed after the first fixed of them. A platform may pass
// a variable argument differently to a named one, so the call is wrong on such
// a platform without this.
func (vm *VM) Variadic(fixed int) {
	vm.fixed = C.int(fixed)
}

func (vm *VM) Free() {
	C.dcFree((*C.DCCallVM)(vm.ptr))
}

func (vm *VM) PushBool(value bool) {
	var val C.DCValue
	if value {
		*(*C.DCbool)(unsafe.Pointer(&val)) = C.DCbool(1)
	} else {
		*(*C.DCbool)(unsafe.Pointer(&val)) = C.DCbool(0)
	}
	vm.buf = append(vm.buf, C.GoArg{
		vtype: C.DC_SIGCHAR_BOOL,
		value: val,
	})
}

func (vm *VM) PushChar(value int8) {
	var val C.DCValue
	*(*C.DCchar)(unsafe.Pointer(&val)) = C.DCchar(value)
	vm.buf = append(vm.buf, C.GoArg{
		vtype: C.DC_SIGCHAR_CHAR,
		value: val,
	})
}

func (vm *VM) PushUnsignedChar(value uint8) {
	var val C.DCValue
	*(*C.DCuchar)(unsafe.Pointer(&val)) = C.DCuchar(value)
	vm.buf = append(vm.buf, C.GoArg{
		vtype: C.DC_SIGCHAR_UCHAR,
		value: val,
	})
}

func (vm *VM) PushShort(value int16) {
	var val C.DCValue
	*(*C.DCshort)(unsafe.Pointer(&val)) = C.DCshort(value)
	vm.buf = append(vm.buf, C.GoArg{
		vtype: C.DC_SIGCHAR_SHORT,
		value: val,
	})
}

func (vm *VM) PushUnsignedShort(value uint16) {
	var val C.DCValue
	*(*C.DCushort)(unsafe.Pointer(&val)) = C.DCushort(value)
	vm.buf = append(vm.buf, C.GoArg{
		vtype: C.DC_SIGCHAR_USHORT,
		value: val,
	})
}

func (vm *VM) PushSignedInt(value int32) {
	var val C.DCValue
	*(*C.DCint)(unsafe.Pointer(&val)) = C.DCint(value)
	vm.buf = append(vm.buf, C.GoArg{
		vtype: C.DC_SIGCHAR_INT,
		value: val,
	})
}

func (vm *VM) PushSignedLong(value int) {
	var val C.DCValue
	*(*C.DClong)(unsafe.Pointer(&val)) = C.DClong(value)
	vm.buf = append(vm.buf, C.GoArg{
		vtype: C.DC_SIGCHAR_LONG,
		value: val,
	})
}

func (vm *VM) PushSignedLongLong(value int64) {
	var val C.DCValue
	*(*C.DClonglong)(unsafe.Pointer(&val)) = C.DClonglong(value)
	vm.buf = append(vm.buf, C.GoArg{
		vtype: C.DC_SIGCHAR_LONGLONG,
		value: val,
	})
}

func (vm *VM) PushFloat(value float32) {
	var val C.DCValue
	*(*C.DCfloat)(unsafe.Pointer(&val)) = C.DCfloat(value)
	vm.buf = append(vm.buf, C.GoArg{
		vtype: C.DC_SIGCHAR_FLOAT,
		value: val,
	})
}

func (vm *VM) PushDouble(value float64) {
	var val C.DCValue
	*(*C.DCdouble)(unsafe.Pointer(&val)) = C.DCdouble(value)
	vm.buf = append(vm.buf, C.GoArg{
		vtype: C.DC_SIGCHAR_DOUBLE,
		value: val,
	})
}

// PushPointer stores value as plain bytes, which the garbage collector cannot
// see, so a Go pointer must be pinned by the caller until the call returns.
func (vm *VM) PushPointer(value unsafe.Pointer) {
	var val C.DCValue
	// Go sees val as a byte array, which isn't aligned for a value of pointer type.
	*(*C.uintptr_t)(unsafe.Pointer(&val)) = C.uintptr_t(uintptr(value))
	vm.buf = append(vm.buf, C.GoArg{
		vtype: C.DC_SIGCHAR_POINTER,
		value: val,
	})
}

func (vm *VM) Call(address unsafe.Pointer) {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)), vm.fixed)
	C.dcCallVoid((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address)))
}

func (vm *VM) CallBool(address unsafe.Pointer) bool {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)), vm.fixed)
	return C.dcCallBool((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))) != 0
}

func (vm *VM) CallChar(address unsafe.Pointer) int8 {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)), vm.fixed)
	return int8(C.dcCallChar((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))))
}

func (vm *VM) CallShort(address unsafe.Pointer) int16 {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)), vm.fixed)
	return int16(C.dcCallShort((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))))
}

func (vm *VM) CallInt(address unsafe.Pointer) int32 {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)), vm.fixed)
	return int32(C.dcCallInt((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))))
}

func (vm *VM) CallLong(address unsafe.Pointer) int {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)), vm.fixed)
	return int(C.dcCallLong((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))))
}

func (vm *VM) CallLongLong(address unsafe.Pointer) int64 {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)), vm.fixed)
	return int64(C.dcCallLongLong((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))))
}

func (vm *VM) CallFloat(address unsafe.Pointer) float32 {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)), vm.fixed)
	return float32(C.dcCallFloat((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))))
}

func (vm *VM) CallDouble(address unsafe.Pointer) float64 {
	return float64(C.goCallDouble((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address)), unsafe.SliceData(vm.buf), C.int(len(vm.buf)), vm.fixed))
}

func (vm *VM) CallPointer(address unsafe.Pointer) unsafe.Pointer {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)), vm.fixed)
	return unsafe.Pointer(C.dcCallPointer((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))))
}
