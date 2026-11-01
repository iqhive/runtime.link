//go:build !cgo || windows

package dll

import (
	"errors"
	"unsafe"
)

type SymbolTable unsafe.Pointer

func Sqrt(f float64) float64 {
	return 0
}

func Open(filename string) (SymbolTable, error) {
	return nil, errors.New("dll: dynamic loading needs CGO and dlfcn.h")
}

func Sym(table SymbolTable, symbol string) (unsafe.Pointer, error) {
	return nil, errors.New("dll: dynamic loading needs CGO and dlfcn.h")
}
