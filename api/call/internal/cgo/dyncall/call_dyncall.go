//go:build cgo

package dyncall

/*
#include <assert.h>
#include <dyncall.h>
#include <dyncall_callback.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <memory.h>

extern DCsigchar bridge_callback(DCCallback*, DCArgs*, DCValue*, uintptr_t);

DCCallback *goNewCallback(const DCsigchar * signature, uintptr_t userdata) {
	return dcbNewCallback(signature, (DCCallbackHandler*)bridge_callback, (void*)userdata);
}

const unsigned int Invalid = 0;
const unsigned int Bool = 1;
const unsigned int Int = 2;
const unsigned int Int8 = 3;
const unsigned int Int16 = 4;
const unsigned int Int32 = 5;
const unsigned int Int64 = 6;
const unsigned int Uint = 7;
const unsigned int Uint8 = 8;
const unsigned int Uint16 = 9;
const unsigned int Uint32 = 10;
const unsigned int Uint64 = 11;
const unsigned int Uintptr = 12;
const unsigned int Float32 = 13;
const unsigned int Float64 = 14;
const unsigned int Complex64 = 15;
const unsigned int Complex128 = 16;
const unsigned int Array = 17;
const unsigned int Chan = 18;
const unsigned int Func = 19;
const unsigned int Interface = 20;
const unsigned int Map = 21;
const unsigned int Pointer = 22;
const unsigned int Slice = 23;
const unsigned int String = 24;
const unsigned int Struct = 25;
const unsigned int UnsafePointer = 26;

_Thread_local DCCallVM *vm;

typedef struct {
	void *fn;
	unsigned long rtype;
	signed long count;
	unsigned long kinds[10];
} Returns;

static inline void call(Returns *fn, void *result, void **args) {
	if (__builtin_expect(vm == NULL, 0)) vm = dcNewCallVM(4096);
	dcReset(vm);
	for (int i = 0; i < fn->count; i++) {
		switch (fn->kinds[i]) {
		case Invalid:								break;
		case Bool: case Int8: case Uint8:
			dcArgChar(vm, *(char*)args[i]); 		break;
		case Int16: case Uint16:
			dcArgShort(vm, *((short *)args[i])); 	break;
		case Int32: case Uint32:
			dcArgInt(vm, *((int *)args[i])); 		break;
		case Int64: case Uint64: case Int: case Uint:
			dcArgLong(vm, *((long *)args[i])); 	break;
		case Float32:
			dcArgFloat(vm, *((float *)args[i])); 	break;
		case Float64:
			dcArgDouble(vm, *((double *)args[i]));break;
		case UnsafePointer: case Uintptr: case Pointer: case String: case Slice: case Func: case Chan: case Map: case Interface:
	 		dcArgPointer(vm, (void *)args[i]); 	break;
		}
	}
	switch (fn->rtype) {
	case Invalid:
		dcCallVoid(vm, fn->fn); 					break;
	case Bool: case Int8: case Uint8:
 		*(char*)result 	= dcCallChar(vm, fn->fn); 	break;
   	case Int16: case Uint16:
    	*(short*)result = dcCallShort(vm, fn->fn); 	break;
    case Int32: case Uint32:
    	*(int*)result 	= dcCallInt(vm, fn->fn); 	break;
    case Int64: case Uint64: case Int: case Uint:
    	*(long*)result 	= dcCallLong(vm, fn->fn); 	break;
    case Float32:
    	*(float*)result = dcCallFloat(vm, fn->fn); 	break;
    case Float64:
    	*(double*)result = dcCallDouble(vm, fn->fn);break;
    case UnsafePointer: case Uintptr:
    	*(void**)result = dcCallPointer(vm, fn->fn);break;
	}
}

#cgo noescape call0
static inline void call0(Returns *fn, void *result) {
	call(fn, result, NULL);
}
#cgo noescape call1
static inline void call1(Returns *fn, void *result, void *arg1) {
	void *args[] = {arg1};
	call(fn, result, &args[0]);
}
#cgo noescape call2
static inline void call2(Returns *fn, void *result, void *arg1, void *arg2) {
	void *args[] = {arg1, arg2};
	call(fn, result, &args[0]);
}
#cgo noescape call3
static inline void call3(Returns *fn, void *result, void *arg1, void *arg2, void *arg3) {
	void *args[] = {arg1, arg2, arg3};
	call(fn, result, &args[0]);
}
#cgo noescape call4
static inline void call4(Returns *fn, void *result, void *arg1, void *arg2, void *arg3, void *arg4) {
	void *args[] = {arg1, arg2, arg3, arg4};
	call(fn, result, &args[0]);
}
#cgo noescape call5
static inline void call5(Returns *fn, void *result, void *arg1, void *arg2, void *arg3, void *arg4, void *arg5) {
	void *args[] = {arg1, arg2, arg3, arg4, arg5};
	call(fn, result, &args[0]);
}
#cgo noescape call6
static inline void call6(Returns *fn, void *result, void *arg1, void *arg2, void *arg3, void *arg4, void *arg5, void *arg6) {
	void *args[] = {arg1, arg2, arg3, arg4, arg5, arg6};
	call(fn, result, &args[0]);
}
#cgo noescape call7
static inline void call7(Returns *fn, void *result, void *arg1, void *arg2, void *arg3, void *arg4, void *arg5, void *arg6, void *arg7) {
	void *args[] = {arg1, arg2, arg3, arg4, arg5, arg6, arg7};
	call(fn, result, &args[0]);
}
#cgo noescape call8
static inline void call8(Returns *fn, void *result, void *arg1, void *arg2, void *arg3, void *arg4, void *arg5, void *arg6, void *arg7, void *arg8) {
	void *args[] = {arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8};
	call(fn, result, &args[0]);
}
#cgo noescape call9
static inline void call9(Returns *fn, void *result, void *arg1, void *arg2, void *arg3, void *arg4, void *arg5, void *arg6, void *arg7, void *arg8, void *arg9) {
	void *args[] = {arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8, arg9};
	call(fn, result, &args[0]);
}
#cgo noescape call10
static inline void call10(Returns *fn, void *result, void *arg1, void *arg2, void *arg3, void *arg4, void *arg5, void *arg6, void *arg7, void *arg8, void *arg9, void *arg10) {
	void *args[] = {arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8, arg9, arg10};
	call(fn, result, &args[0]);
}

static inline void* get_trampoline() { return call; }

*/
import "C"
import (
	"unsafe"
)

type Callback C.DCCallback

type CallbackHandler func(*Callback, *Args, unsafe.Pointer) rune

func NewCallback(sig Signature, handler CallbackHandler) *Callback {
	functions = append(functions, handler)

	s := C.CString(string(sig.Args) + ")" + string(sig.Returns))
	defer C.free(unsafe.Pointer(s))
	return (*Callback)(C.goNewCallback((*C.DCsigchar)(s), C.uintptr_t(len(functions))))
}

func (callback *Callback) Free() {
	C.dcbFreeCallback((*C.DCCallback)(callback))
}

//go:linkname Slow runtime.link/api/call/internal/cgo/dyncall.slow
//go:noescape
func Slow(fn unsafe.Pointer, ret unsafe.Pointer, args ...unsafe.Pointer)

func slow(fn unsafe.Pointer, ret unsafe.Pointer, args ...unsafe.Pointer) {
	switch len(args) {
	case 0:
		C.call0((*C.Returns)(fn), ret)
	case 1:
		C.call1((*C.Returns)(fn), ret, args[0])
	case 2:
		C.call2((*C.Returns)(fn), ret, args[0], args[1])
	case 3:
		C.call3((*C.Returns)(fn), ret, args[0], args[1], args[2])
	case 4:
		C.call4((*C.Returns)(fn), ret, args[0], args[1], args[2], args[3])
	case 5:
		C.call5((*C.Returns)(fn), ret, args[0], args[1], args[2], args[3], args[4])
	case 6:
		C.call6((*C.Returns)(fn), ret, args[0], args[1], args[2], args[3], args[4], args[5])
	case 7:
		C.call7((*C.Returns)(fn), ret, args[0], args[1], args[2], args[3], args[4], args[5], args[6])
	case 8:
		C.call8((*C.Returns)(fn), ret, args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7])
	case 9:
		C.call9((*C.Returns)(fn), ret, args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8])
	case 10:
		C.call10((*C.Returns)(fn), ret, args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9])
	}
}

func GetTrampoline() unsafe.Pointer {
	return C.get_trampoline()
}
