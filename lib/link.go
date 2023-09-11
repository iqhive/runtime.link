package lib

import (
	"fmt"
	"runtime"
	"unsafe"

	"runtime.link/lib/internal/dll"
	"runtime.link/std"
	"runtime.link/std/abi"
)

// Import the given library, using the additionally provided
// locations to search for the library.
func Import[Library any](locations ...string) Library {
	var lib Library
	var structure = std.StructureOf(&lib)
	locations = append(locations, structure.Host.Get("lib"))
	for _, name := range locations {
		symbols, err := dll.Open(name)
		if err != nil {
			continue
		}
		link(structure, symbols)
		return lib
	}
	structure.MakeError(fmt.Errorf("library for %T not available on %s", lib, runtime.GOOS))
	return lib
}

func link(structure std.Structure, symbols dll.SymbolTable) {
linking:
	for _, fn := range structure.Functions {
		fn := fn
		tag := Tag(fn.Tags.Get("lib"))
		if tag == "" {
			continue linking
		}
		var symbol unsafe.Pointer
		names, stype, err := tag.Parse()
		if err != nil {
			fn.MakeError(err)
			continue linking
		}
		for _, name := range names {
			symbol, err = dll.Sym(symbols, name)
			if err != nil {
				fn.MakeError(err)
				continue linking
			}
		}
		src, err := compile(fn.Type, stype)
		if err != nil {
			fmt.Println(fn.Name, fn.Type, src, tag)
			fmt.Println(err)
			fmt.Println()
			fn.MakeError(err)
			continue linking
		}
		call, err := abi.Default.Call(fn.Type, symbol, src)
		if err != nil {
			fn.MakeError(err)
			continue
		}
		fn.Make(call)
	}
	for _, structure := range structure.Namespace {
		link(structure, symbols)
	}
}
