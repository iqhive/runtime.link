//go:build !cgo

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
	return nil, errors.New("dll: CGO is disabled")
}

func Sym(table SymbolTable, symbol string) (unsafe.Pointer, error) {
	return nil, errors.New("dll: CGO is disabled")
}
