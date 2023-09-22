// Package dll provides methods for dynamically loading shared libraries and symbol lookup.
package dll

/*
#cgo linux LDFLAGS: -lm
#include <dlfcn.h>
#include <stdlib.h>
#include <math.h>
*/
import "C"
import (
	"errors"
	"unsafe"
)

// Sqrt used for benchmarking.
func Sqrt(f float64) float64 {
	return float64(C.sqrt(C.double(f)))
}

// SymbolTable pointer.
type SymbolTable unsafe.Pointer

// Open the named library and return a handle to the symbol table.
func Open(filename string) (SymbolTable, error) {
	ptr := dlopen(filename)
	if ptr == nil {
		return nil, errors.New(dlerror())
	}
	return SymbolTable(ptr), nil
}

// Sym returns the address of the named symbol in the given symbol table.
func Sym(table SymbolTable, symbol string) (unsafe.Pointer, error) {
	ptr := dlsym(unsafe.Pointer(table), symbol)
	if ptr == nil {
		return nil, errors.New(dlerror())
	}
	return ptr, nil
}

func dlopen(filename string) (handle unsafe.Pointer) {
	s := C.CString(filename + "\x00")
	defer C.free(unsafe.Pointer(s))
	return C.dlopen(s, C.RTLD_NOW)
}

func dlerror() string {
	return C.GoString(C.dlerror())
}

func dlsym(handle unsafe.Pointer, symbol string) unsafe.Pointer {
	s := C.CString(symbol + "\x00")
	defer C.free(unsafe.Pointer(s))
	return C.dlsym(handle, s)
}
