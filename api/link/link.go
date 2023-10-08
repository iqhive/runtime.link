package link

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"unsafe"

	"runtime.link/api"
	"runtime.link/api/link/internal/dll"
	"runtime.link/api/link/internal/ffi"
	"runtime.link/jit"
)

// ABI map to use when calling function, 'abi' struct tag will determine which [jit.ABI] to use.
type ABI map[string]jit.ABI

// API transport implements [api.Linker].
var API api.Linker[string, ABI] = linker{}

type linker struct{}

func (linker) Link(structure api.Structure, lib string, abi ABI) error {
	if abi == nil {
		abi = ABI{
			"": platform{},
		}
	}
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
	link(abi, structure, tables)
	return nil
}

func link(abi ABI, structure api.Structure, tables []dll.SymbolTable) {
	for _, fn := range structure.Functions {
		fn := fn
		tag := fn.Tags.Get("link")
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
		method, ok := abi[fn.Tags.Get("abi")]
		if !ok {
			fn.MakeError(fmt.Errorf("abi %s not found", fn.Tags.Get("abi")))
			continue
		}
		compiled, err := compile(symbol, method, fn.Type, stype)
		if err != nil {
			fn.MakeError(err)
			continue
		}
		fn.Make(compiled)
	}
	for _, structure := range structure.Namespace {
		link(abi, structure, tables)
	}
}

func kindOf(c *ffi.Type) (reflect.Type, error) {
	if c == nil {
		return nil, nil
	}
	switch c.Name {
	case "double":
		return reflect.TypeOf(float64(0)), nil
	default:
		return nil, fmt.Errorf("link currently unsupports %s", c.Name)
	}
}

func compile(symbol unsafe.Pointer, abi jit.ABI, goType reflect.Type, ldType ffi.Type) (reflect.Value, error) {
	return jit.MakeFunc(goType, func(asm jit.Assembly, args []jit.Value) ([]jit.Value, error) {
		var send = make([]jit.Value, len(ldType.Args))
		for i, arg := range ldType.Args {
			var (
				from  = goType.In(arg.Maps - 1)
				value = args[arg.Maps-1]
			)
			switch from.Kind() {
			case reflect.Float64:
				switch arg.Name {
				case "double":
					send[i] = value
				default:
					return nil, fmt.Errorf("link currently unsupports %s", arg.Name)
				}
			default:
				return nil, fmt.Errorf("link currently unsupports %s", from.Kind())
			}
		}
		kind, err := kindOf(ldType.Func)
		if err != nil {
			return nil, err
		}
		call, err := asm.UnsafeCall(abi, symbol, send, kind)
		if err != nil {
			return nil, err
		}
		rets := make([]jit.Value, goType.NumOut())
		if ldType.Func != nil {
			switch ldType.Func.Name {
			case "double":
				rets[0] = call[0]
			default:
				return nil, fmt.Errorf("link currently unsupports %s", ldType.Func.Name)
			}
		}
		return rets, nil
	})
}
