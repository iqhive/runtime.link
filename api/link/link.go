package link

import (
	"fmt"
	"runtime"
	"strings"
	"unsafe"

	"runtime.link/api"
	"runtime.link/api/link/internal/dll"
	"runtime.link/api/link/internal/ffi"
)

// Mode is used to specify how the library should be linked.
type Mode bool

const (
	// CGO + reflect.MakeFunc will be used to create the functions (safest but slowest).
	CGO Mode = false
	// JIT will be used where possible to compile efficient trampolines for each function (experimental).
	// Any functions that cannot be compiled this way, will fall back to using the CGO mode.
	JIT Mode = true
)

// API transport implements [api.Linker].
var API api.Linker[string, Mode] = linker{}

type linker struct{}

func (linker) Link(structure api.Structure, lib string, mode Mode) error {
	var tables []dll.SymbolTable
	if lib == "" {
		lib = structure.Host.Get("lib")
	}
	for _, name := range strings.Split(lib, " ") {
		table, err := dll.Open(name)
		if err != nil {
			continue
		}
		tables = append(tables, table)
	}
	if len(tables) == 0 {
		return fmt.Errorf("library for %T not available on %s", lib, runtime.GOOS)
	}
	link(structure, tables)
	return nil
}

func link(structure api.Structure, tables []dll.SymbolTable) {
	for _, fn := range structure.Functions {
		fn := fn
		tag := fn.Tags.Get("ffi")
		if tag == "" {
			continue
		}
		var symbol unsafe.Pointer
		names, stype, err := ffi.ParseTag(tag)
		if err != nil {
			fn.MakeError(err)
			continue
		}

		for _, table := range tables {
			for _, name := range names {
				symbol, err = dll.Sym(table, name)
				if err != nil {
					continue
				}
			}
		}
		if symbol == nil {
			fn.MakeError(err)
			continue
		}
		func() {
			defer func() {
				if err := recover(); err != nil {
					fn.MakeError(fmt.Errorf("%s: %v", fn.Name, err))
				}
			}()
			src, err := ffi.CompileForSpeed(fn.Type, stype)
			if err != nil {
				/*fmt.Println(fn.Name, fn.Type, src, tag)
				fmt.Println(err)
				fmt.Println()*/
				fn.MakeError(err)
				return
			}
			src.Call = symbol
			fn.Make(src.MakeFunc(fn.Type))
		}()
	}
	for _, structure := range structure.Namespace {
		link(structure, tables)
	}
}
