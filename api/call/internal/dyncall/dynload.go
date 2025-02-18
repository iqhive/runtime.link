package dyncall

// #include "dynload/dynload.h"
// #include <stdlib.h>
import "C"
import (
	"unsafe"
)

func LoadLibrary(libname string) Library {
	s := C.CString(libname)
	defer C.free(unsafe.Pointer(s))
	return Library(C.dlLoadLibrary(s))
}

func FindSymbol(lib Library, symbol string) unsafe.Pointer {
	s := C.CString(symbol)
	defer C.free(unsafe.Pointer(s))
	return unsafe.Pointer(C.dlFindSymbol((*C.DLLib)(lib), s))
}

type Library unsafe.Pointer
