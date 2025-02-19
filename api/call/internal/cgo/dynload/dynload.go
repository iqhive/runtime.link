package dynload

// #include "dynload.h"
// #include <stdlib.h>
import "C"
import (
	"unsafe"
)

func Library(libname string) LibraryPointer {
	s := C.CString(libname)
	defer C.free(unsafe.Pointer(s))
	return LibraryPointer(C.dlLoadLibrary(s))
}

func FindSymbol(lib LibraryPointer, symbol string) unsafe.Pointer {
	s := C.CString(symbol)
	defer C.free(unsafe.Pointer(s))
	return unsafe.Pointer(C.dlFindSymbol((*C.DLLib)(lib), s))
}

type LibraryPointer unsafe.Pointer
