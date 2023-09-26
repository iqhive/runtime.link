package lib

import (
	"fmt"
	"runtime"
	"strings"
	"unsafe"

	"runtime.link/lib/internal/dll"
	"runtime.link/lib/internal/ffi"
	"runtime.link/qnq"
)

// Import the given library, using the additionally provided
// locations to search for the library.
func Import[Library any](locations ...string) Library {
	var lib Library
	var structure = qnq.StructureOf(&lib)
	locations = append(locations, structure.Host.Get("lib"))
	for _, names := range locations {
		var tables []dll.SymbolTable
		for _, name := range strings.Split(names, " ") {
			table, err := dll.Open(name)
			if err != nil {
				continue
			}
			tables = append(tables, table)
		}
		if len(tables) == 0 {
			continue
		}
		link(structure, tables)
		return lib
	}
	structure.MakeError(fmt.Errorf("library for %T not available on %s", lib, runtime.GOOS))
	return lib
}

func link(structure qnq.Structure, tables []dll.SymbolTable) {
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
