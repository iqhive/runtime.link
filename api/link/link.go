package link

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"unsafe"

	"runtime.link/api"
	"runtime.link/api/link/internal/cgo"
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

func normal(kind reflect.Kind) reflect.Type {
	switch kind {
	case reflect.Int8:
		return reflect.TypeOf(int8(0))
	case reflect.Int16:
		return reflect.TypeOf(int16(0))
	case reflect.Int32:
		return reflect.TypeOf(int32(0))
	case reflect.Int:
		return reflect.TypeOf(int(0))
	case reflect.Int64:
		return reflect.TypeOf(int64(0))
	case reflect.Uint8:
		return reflect.TypeOf(uint8(0))
	case reflect.Uint16:
		return reflect.TypeOf(uint16(0))
	case reflect.Uint32:
		return reflect.TypeOf(uint32(0))
	case reflect.Uint:
		return reflect.TypeOf(uint(0))
	case reflect.Uint64:
		return reflect.TypeOf(uint64(0))
	case reflect.Uintptr:
		return reflect.TypeOf(uintptr(0))
	case reflect.Float32:
		return reflect.TypeOf(float32(0))
	case reflect.Float64:
		return reflect.TypeOf(float64(0))
	case reflect.Complex64:
		return reflect.TypeOf(complex64(0))
	case reflect.Complex128:
		return reflect.TypeOf(complex128(0))
	case reflect.Bool:
		return reflect.TypeOf(false)
	case reflect.String:
		return reflect.TypeOf("")
	case reflect.UnsafePointer:
		return reflect.TypeOf(unsafe.Pointer(nil))
	default:
		return nil
	}
}

func compile(symbol unsafe.Pointer, abi jit.ABI, goType reflect.Type, ldType ffi.Type) (reflect.Value, error) {
	return jit.MakeFunc(goType, func(asm jit.Assembly, args []jit.Value) ([]jit.Value, error) {
		var pinner = asm.Pinner()
		defer pinner.Unpin()
		var send = make([]jit.Value, len(ldType.Args))
		for i, arg := range ldType.Args {
			var (
				from  = goType.In(arg.Maps - 1)
				value = args[arg.Maps-1]
			)
			into := cgo.Types.LookupKind(arg.Name)
			if from.Kind() == into {
				send[i] = value
			} else {
				if from.ConvertibleTo(normal(into)) {
					send[i] = asm.Convert(value, normal(into))
					continue
				}
				if from.Kind() == reflect.String && arg.Name == "char" && arg.Free == '&' {
					s := asm.NullTerminated(value)
					pinner.Pin(s)
					send[i] = s.UnsafePointer()
					continue
				}
				return nil, fmt.Errorf("link currently unsupports %s arguments", from.Kind())
			}
		}
		var kind reflect.Type
		if ldType.Func != nil {
			kind = normal(cgo.Types.LookupKind(ldType.Func.Name))
		}
		call, _, err := asm.UnsafeCall(abi, symbol, send, kind)
		if err != nil {
			return nil, err
		}
		rets := make([]jit.Value, goType.NumOut())
		if ldType.Func != nil {
			into := goType.Out(0)
			from := cgo.Types.LookupKind(ldType.Func.Name)
			if into.Kind() == from {
				rets[0] = call[0]
			} else {
				if normal(from).ConvertibleTo(goType.Out(0)) {
					rets[0] = asm.Convert(call[0], goType.Out(0))
				} else {
					switch into.Kind() {
					case reflect.Bool:
						rets[0] = asm.Not(asm.IsZero(call[0]))
					case reflect.Interface:
						if into == reflect.TypeOf((*error)(nil)).Elem() {
							rets[0] = asm.NewError()
						} else {
							return nil, fmt.Errorf("link currently unsupports %s results", ldType.Func.Name)
						}
					default:
						return nil, fmt.Errorf("link currently unsupports %s results", ldType.Func.Name)
					}
				}
			}
		}
		return rets, nil
	})
}
