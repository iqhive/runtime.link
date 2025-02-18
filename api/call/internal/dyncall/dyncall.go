//go:build cgo

package dyncall

/*
#include "dyncall/dyncall.h"
#include "dyncall/dyncall_callback.h"
*/
import "C"
import (
	"unsafe"
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

var functions []CallbackHandler

//export bridge_callback
func bridge_callback(cb *C.DCCallback, args *C.DCArgs, result unsafe.Pointer, userdata uintptr) C.DCsigchar {
	return C.DCsigchar(functions[userdata-1]((*Callback)(cb), (*Args)(args), result))
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
