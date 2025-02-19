//go:build cgo

package dyncall

/*
#cgo noescape call_1
#cgo nocallback call_1

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

typedef struct {
	DCsigchar vtype;
	DCValue value;
} GoArg;

void goArgs(DCCallVM *vm, GoArg *arg, int argc) {
	dcReset(vm);
	DCValue value;
	for (int i = 0; i < argc; i++) {
		value = arg[i].value;
		switch (arg[i].vtype) {
		case DC_SIGCHAR_BOOL:
			dcArgBool(vm, value.B);
			break;
		case DC_SIGCHAR_CHAR:
			dcArgChar(vm, value.c);
			break;
		case DC_SIGCHAR_UCHAR:
			dcArgChar(vm, value.c);
			break;
		case DC_SIGCHAR_SHORT:
			dcArgShort(vm, value.s);
			break;
		case DC_SIGCHAR_USHORT:
			dcArgShort(vm, value.s);
			break;
		case DC_SIGCHAR_INT:
			dcArgInt(vm, value.i);
			break;
		case DC_SIGCHAR_UINT:
			dcArgInt(vm, value.i);
			break;
		case DC_SIGCHAR_LONG:
			dcArgLong(vm, value.l);
			break;
		case DC_SIGCHAR_ULONG:
			dcArgLong(vm, value.J);
			break;
		case DC_SIGCHAR_LONGLONG:
			dcArgLongLong(vm, value.l);
			break;
		case DC_SIGCHAR_ULONGLONG:
			dcArgLongLong(vm, value.l);
			break;
		case DC_SIGCHAR_FLOAT:
			dcArgFloat(vm, value.f);
			break;
		case DC_SIGCHAR_DOUBLE:
			dcArgDouble(vm, value.d);
			break;
		case DC_SIGCHAR_POINTER:
			dcArgPointer(vm, value.p);
			break;
		case DC_SIGCHAR_STRING:
			dcArgPointer(vm, value.p);
			break;
		case DC_SIGCHAR_AGGREGATE:
			assert(0); // FIXME
			break;
		}
	}
}

double goCallDouble(DCCallVM *vm, DCpointer funcptr, GoArg *arg, int argc) {
	goArgs(vm, arg, argc);
	return dcCallDouble(vm, funcptr);
}


const uint8_t Ignored = 0;
const uint8_t Binary1 = 1;
const uint8_t Binary2 = 2;
const uint8_t Binary4 = 3;
const uint8_t Binary8 = 4;
const uint8_t Float32 = 5;
const uint8_t Float64 = 6;
const uint8_t Pointer = 7;
const uint8_t Repeats = 8;
const uint8_t Offsets = 9;

_Thread_local DCCallVM *vm;

static inline void standard_call(void *fn, void *callframe[], uint8_t codes[], uint32_t length) {
	if (__builtin_expect(length == 0, 0)) return;
	if (__builtin_expect(vm == NULL, 0)) vm = dcNewCallVM(4096);
	dcReset(vm);
	int arg = 1;
	for (int i = 1; i < length; i++) {
		switch (codes[i]) {
		case Ignored:												break;
		case Binary1: dcArgChar(vm, *(char*)callframe[arg]); 		break;
		case Binary2: dcArgShort(vm, *((short *)callframe[arg])); 	break;
		case Binary4: dcArgInt(vm, *((int *)callframe[arg])); 		break;
		case Binary8: dcArgLong(vm, *((long *)callframe[arg])); 	break;
		case Float32: dcArgFloat(vm, *((float *)callframe[arg])); 	break;
		case Float64: dcArgDouble(vm, *((double *)callframe[arg])); break;
		case Pointer: dcArgPointer(vm, (void *)callframe[arg]); 	break;
		}
		arg++;
	}
	switch (codes[0]) {
	case Ignored: dcCallVoid(vm, fn); 								break;
	case Binary1: *(char*)callframe[0] = dcCallChar(vm, fn); 		break;
	case Binary2: *(short*)callframe[0]  = dcCallShort(vm, fn); 	break;
	case Binary4: *(int*)callframe[0]  = dcCallInt(vm, fn); 		break;
	case Binary8: *(long*)callframe[0]  = dcCallLongLong(vm, fn); 	break;
	case Float32: *(float*)callframe[0]  = dcCallFloat(vm, fn);		break;
	case Float64: *(double*)callframe[0]  = dcCallDouble(vm, fn);	break;
	case Pointer: callframe[0] = dcCallPointer(vm, fn); 			break;
	}
}

static inline void trampoline(void *fn, void *args, uint8_t *codes) {
	standard_call(fn, args, codes, 2);
}

static inline void* get_trampoline() { return trampoline; }

static inline void call_1(void *fn, uint8_t codes[], uint32_t length, void *ret1, void *arg1) {
	void *callframe[] = {ret1,arg1};
	standard_call(fn, callframe, codes, length);
}

*/
import "C"
import (
	"unsafe"

	"runtime.link/api/call/callframe"
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

type VM struct {
	ptr *C.DCCallVM
	buf []C.GoArg
}

func NewVM(size int) *VM {
	return &VM{
		ptr: C.dcNewCallVM(C.size_t(size)),
		buf: make([]C.GoArg, 0),
	}
}

func (vm *VM) Reset() {
	vm.buf = vm.buf[:0]
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

func (vm *VM) PushShort(value int16) {
	var val C.DCValue
	*(*C.DCshort)(unsafe.Pointer(&val)) = C.DCshort(value)
	vm.buf = append(vm.buf, C.GoArg{
		vtype: C.DC_SIGCHAR_SHORT,
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

func (vm *VM) PushPointer(value unsafe.Pointer) {
	var val C.DCValue
	*(*C.DCpointer)(unsafe.Pointer(&val)) = C.DCpointer(value)
	vm.buf = append(vm.buf, C.GoArg{
		vtype: C.DC_SIGCHAR_POINTER,
		value: val,
	})
}

func (vm *VM) Call(address unsafe.Pointer) {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)))
	C.dcCallVoid((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address)))
}

func (vm *VM) CallBool(address unsafe.Pointer) bool {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)))
	return C.dcCallBool((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))) != 0
}

func (vm *VM) CallChar(address unsafe.Pointer) int8 {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)))
	return int8(C.dcCallChar((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))))
}

func (vm *VM) CallShort(address unsafe.Pointer) int16 {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)))
	return int16(C.dcCallShort((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))))
}

func (vm *VM) CallInt(address unsafe.Pointer) int32 {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)))
	return int32(C.dcCallInt((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))))
}

func (vm *VM) CallLong(address unsafe.Pointer) int {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)))
	return int(C.dcCallLong((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))))
}

func (vm *VM) CallLongLong(address unsafe.Pointer) int64 {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)))
	return int64(C.dcCallLongLong((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))))
}

func (vm *VM) CallFloat(address unsafe.Pointer) float32 {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)))
	return float32(C.dcCallFloat((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))))
}

func (vm *VM) CallDouble(address unsafe.Pointer) float64 {
	return float64(C.goCallDouble((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address)), unsafe.SliceData(vm.buf), C.int(len(vm.buf))))
}

func (vm *VM) CallPointer(address unsafe.Pointer) unsafe.Pointer {
	C.goArgs((*C.DCCallVM)(vm.ptr), unsafe.SliceData(vm.buf), C.int(len(vm.buf)))
	return unsafe.Pointer(C.dcCallPointer((*C.DCCallVM)(vm.ptr), (C.DCpointer)(unsafe.Pointer(address))))
}

func Standard(fn unsafe.Pointer, ret unsafe.Pointer, args ...unsafe.Pointer) {
	switch len(args) {
	case 1:
		var codes = [...]callframe.Code{
			callframe.Float64,
			callframe.Float64,
		}
		C.call_1(fn, (*C.uint8_t)(&codes[0]), 2, ret, args[0])
	default:
		panic("unsupported number of arguments")
	}
}

func GetTrampoline() unsafe.Pointer {
	return C.get_trampoline()
}
