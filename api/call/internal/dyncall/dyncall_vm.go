//go:build cgo

package dyncall

/*
#include <assert.h>
#include "dyncall/dyncall.h"
#include "dyncall/dyncall_callback.h"
#include <stdint.h>
#include <stdlib.h>

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
