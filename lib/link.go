package lib

import (
	"fmt"
	"reflect"
	"runtime"
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
		names, _, err := tag.Parse()
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
		var slow error
		for i := 0; i < fn.Type.NumIn(); i++ {
			arg := fn.Type.In(i)
			switch arg.Kind() {
			case reflect.Bool:
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			case reflect.Float32, reflect.Float64:
			default:
				slow = fmt.Errorf("unsupported argument type %s for %s", arg, fn.Name)
			}
		}
		if fn.Type.NumOut() > 0 {
			switch fn.Type.Out(0).Kind() {
			case reflect.Bool:
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			case reflect.Float32, reflect.Float64:
			default:
				slow = fmt.Errorf("unsupported return type %s for %s", fn.Type.Out(0), fn.Name)
			}
		}
		if slow != nil {
			fn.MakeError(slow)
			continue linking
		}
		/*direct := func(args asm.Registers) asm.Registers {
			args = asm.Call(symbol, args)
			return args
		}*/
		direct := &symbol
		fn.Make(reflect.NewAt(fn.Type, reflect.ValueOf(&direct).UnsafePointer()).Elem())
	}
	for _, structure := range structure.Namespace {
		link(structure, symbols)
	}
}
