//go:build cgo

package dyncall

/*
#include <stdint.h>
#include "dyncall/dyncall.h"
#include "dyncall/dyncall_callback.h"

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

static inline void standard_call(void *stack, void *fn, void *callframe[], uint8_t codes[], uint32_t length) {
	if (length == 0) {
		return;
	}
	DCCallVM *vm = dcNewCallVM(4096);
	int arg = 0;
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
	case Binary1: *(char*)callframe = dcCallChar(vm, fn); 		break;
	case Binary2: *(short*)callframe = dcCallShort(vm, fn); 		break;
	case Binary4: *(int*)callframe = dcCallInt(vm, fn); 			break;
	case Binary8: *(long*)callframe = dcCallLongLong(vm, fn); 	break;
	case Float32: *(float*)callframe = dcCallFloat(vm, fn); 		break;
	case Float64: *(double*)callframe = dcCallDouble(vm, fn); 	break;
	case Pointer: callframe[0] = dcCallPointer(vm, fn); 	break;
	}
	dcFree(vm);
}
*/
import "C"
import (
	"unsafe"

	"runtime.link/api/call/callframe"
)

const (
	Void             = C.DC_SIGCHAR_VOID
	Bool             = C.DC_SIGCHAR_BOOL
	Char             = C.DC_SIGCHAR_CHAR
	UnsignedChar     = C.DC_SIGCHAR_UCHAR
	Short            = C.DC_SIGCHAR_SHORT
	UnsignedShort    = C.DC_SIGCHAR_USHORT
	Int              = C.DC_SIGCHAR_INT
	Uint             = C.DC_SIGCHAR_UINT
	Long             = C.DC_SIGCHAR_LONG
	UnsignedLong     = C.DC_SIGCHAR_ULONG
	LongLong         = C.DC_SIGCHAR_LONGLONG
	UnsignedLongLong = C.DC_SIGCHAR_ULONGLONG
	Float            = C.DC_SIGCHAR_FLOAT
	Double           = C.DC_SIGCHAR_DOUBLE
	Pointer          = C.DC_SIGCHAR_POINTER
	String           = C.DC_SIGCHAR_STRING
	Aggregate        = C.DC_SIGCHAR_AGGREGATE
)

type Signature struct {
	Args    []rune
	Returns rune
}

type Args C.DCArgs

func (args *Args) Bool() C.DCbool {
	return C.dcbArgBool((*C.DCArgs)(args))
}

func (args *Args) Char() C.DCchar {
	return C.dcbArgChar((*C.DCArgs)(args))
}

func (args *Args) Short() C.DCshort {
	return C.dcbArgShort((*C.DCArgs)(args))
}

func (args *Args) Int() C.DCint {
	return C.dcbArgInt((*C.DCArgs)(args))
}

func (args *Args) Long() C.DClong {
	return C.dcbArgLong((*C.DCArgs)(args))
}

func (args *Args) LongLong() C.DClonglong {
	return C.dcbArgLongLong((*C.DCArgs)(args))
}

func (args *Args) UnsignedChar() C.DCuchar {
	return C.dcbArgUChar((*C.DCArgs)(args))
}

func (args *Args) UnsignedShort() C.DCushort {
	return C.dcbArgUShort((*C.DCArgs)(args))
}

func (args *Args) UnsignedInt() C.DCuint {
	return C.dcbArgUInt((*C.DCArgs)(args))
}

func (args *Args) UnsignedLong() C.DCulong {
	return C.dcbArgULong((*C.DCArgs)(args))
}

func (args *Args) UnsignedLongLong() C.DCulonglong {
	return C.dcbArgULongLong((*C.DCArgs)(args))
}

func (args *Args) Float() C.DCfloat {
	return C.dcbArgFloat((*C.DCArgs)(args))
}

func (args *Args) Double() C.DCdouble {
	return C.dcbArgDouble((*C.DCArgs)(args))
}

func (args *Args) Pointer() C.DCpointer {
	return C.dcbArgPointer((*C.DCArgs)(args))
}

func Standard(stack []byte, fn unsafe.Pointer, args callframe.Args) {
	codes := args.Codes()
	C.standard_call(unsafe.Pointer(&stack[0]), fn, &args.Pointers()[0], (*C.uint8_t)(&codes[0]), C.uint32_t(len(codes)))
}
