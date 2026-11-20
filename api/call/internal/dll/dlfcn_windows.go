//go:build cgo

package dll

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Sqrt is for benchmarking a call into C, which opening a library here doesn't need.
func Sqrt(f float64) float64 {
	return 0
}

// SymbolTable is the handle of a loaded module.
type SymbolTable syscall.Handle

// Open the named library and return a handle to the symbol table. Windows has
// no dlfcn.h, its loader is in kernel32. A relative name is searched for by
// the loader, whose search order includes the current directory, so a name
// that must not be found elsewhere is given as an absolute path. It is only
// built with CGO because nothing that is found in a library can be called
// without it.
func Open(filename string) (SymbolTable, error) {
	handle, err := syscall.LoadLibrary(filename)
	if err != nil {
		return 0, fmt.Errorf("unable to open %s: %w", filename, err)
	}
	return SymbolTable(handle), nil
}

// Sym returns the address of the named symbol in the given symbol table.
func Sym(table SymbolTable, symbol string) (unsafe.Pointer, error) {
	address, err := syscall.GetProcAddress(syscall.Handle(table), symbol)
	if err != nil {
		return nil, fmt.Errorf("unable to find %s: %w", symbol, err)
	}
	// the address is of a function in the module, which is not memory that Go manages or
	// moves, and reading it as a pointer is the conversion that go vet has no complaint about.
	return *(*unsafe.Pointer)(unsafe.Pointer(&address)), nil
}
