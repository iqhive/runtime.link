package lib

import (
	"fmt"
	"runtime"
	"strings"
	"unsafe"

	"runtime.link/lib/internal/dll"
	"runtime.link/std"
)

// Import the given library, using the additionally provided
// locations to search for the library.
func Import[Library any](locations ...string) Library {
	var lib Library
	var structure = std.StructureOf(&lib)
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

func link(structure std.Structure, tables []dll.SymbolTable) {
	for _, fn := range structure.Functions {
		fn := fn
		tag := Tag(fn.Tags.Get("lib"))
		if tag == "" {
			continue
		}
		var symbol unsafe.Pointer
		names, stype, err := tag.Parse()
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
		src, err := compileOutgoing(fn.Type, stype)
		if err != nil {
			/*fmt.Println(fn.Name, fn.Type, src, tag)
			fmt.Println(err)
			fmt.Println()*/
			fn.MakeError(err)
			continue
		}
		src.Call = symbol
		fn.Make(src.MakeFunc(fn.Type))
	}
	for _, structure := range structure.Namespace {
		link(structure, tables)
	}
}
