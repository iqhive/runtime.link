package call

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"unsafe"

	"runtime.link/api"
	"runtime.link/api/call/internal/cgo"
	"runtime.link/api/call/internal/dll"
	"runtime.link/api/call/internal/ffi"
	"runtime.link/jit"
)

// Options
type Options struct {
	LookupSymbol func(string) (unsafe.Pointer, error)
}

func Make[T any](jump unsafe.Pointer, tag string) (T, error) {
	var fn T
	_, stype, err := ffi.ParseTag(" " + tag)
	if err != nil {
		return fn, err
	}
	compiled, err := compile(tag, jump, platform{}, reflect.TypeOf(fn), stype)
	if err != nil {
		return fn, err
	}
	return compiled.Interface().(T), nil
}

// API transport implements [api.Linker].
var API api.Linker[string, Options] = linker{}

type linker struct{}

func (linker) Link(structure api.Structure, lib string, opts Options) error {
	if opts.LookupSymbol != nil {
		link(opts, structure, nil)
		return nil
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
	link(opts, structure, tables)
	return nil
}

func link(opts Options, structure api.Structure, tables []dll.SymbolTable) {
	for _, fn := range structure.Functions {
		fn := fn
		tag := fn.Tags.Get("call")
		if tag == "" {
			continue
		}
		var symbol unsafe.Pointer
		names, stype, err := ffi.ParseTag(tag)
		if err != nil {
			fn.MakeError(err)
			continue
		}
		var finalName string
		if opts.LookupSymbol != nil {
			for _, name := range names {
				symbol, err = opts.LookupSymbol(name)
				if err != nil {
					continue
				}
				finalName = name
			}
		} else {
			for _, table := range tables {
				for _, name := range names {
					symbol, err = dll.Sym(table, name)
					if err != nil {
						continue
					}
					finalName = name
				}
			}
		}
		if symbol == nil {
			fn.MakeError(err)
			continue
		}

		compiled, err := compile(finalName, symbol, platform{}, fn.Type, stype)
		if err != nil {
			fn.MakeError(err)
			continue
		}
		fn.Make(compiled)
	}
	for _, structure := range structure.Namespace {
		link(opts, structure, tables)
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

func compile(name string, symbol unsafe.Pointer, abi jit.ABI, goType reflect.Type, ldType ffi.Type) (reflect.Value, error) {
	return jit.MakeFunc(goType, func(asm jit.Assembly, args []jit.Value) ([]jit.Value, error) {
		//var pinner = asm.Pinner()
		//defer pinner.Unpin()
		var send = make([]jit.Value, len(ldType.Args))
		for i, arg := range ldType.Args {
			into := cgo.Types.LookupKind(arg.Name)
			if arg.Free == '-' {
				var err error
				send[i], err = inferValue(asm, args, arg, into, goType)
				if err != nil {
					return nil, fmt.Errorf("runtime.link/api/call unable to infer argument %d (%s): %w", i, arg.Name, err)
				}
				continue
			}
			if arg.Maps-1 >= goType.NumIn() {
				return nil, fmt.Errorf("runtime.link/api/call too many arguments for %s (%s)", ldType.Name, arg.Name)
			}
			var (
				from  = goType.In(arg.Maps - 1)
				value = args[arg.Maps-1]
			)
			if from.Kind() == into {
				send[i] = value
			} else {
				if from.ConvertibleTo(normal(into)) {
					send[i] = asm.Convert(value, normal(into))
					continue
				}
				if from.Kind() == reflect.String && arg.Name == "char" && arg.Free == '&' {
					s := asm.NullTerminated(value)
					//pinner.Pin(s)
					send[i] = s.UnsafePointer()
					continue
				}
				if from.Kind() == reflect.Slice && (arg.Name == "void" || arg.Name == "char") && arg.Free == '&' {
					//pinner.Pin(value.UnsafePointer())
					send[i] = value.UnsafePointer()
					continue
				}
				if from.Kind() == reflect.UnsafePointer || from.Kind() == reflect.Ptr {
					//pinner.Pin(value.UnsafePointer())
					send[i] = value.UnsafePointer()
					continue
				}
				if from.Kind() == reflect.Uintptr {
					send[i] = value
					continue
				}
				return nil, fmt.Errorf("runtime.link/api/call does not support '%s' arguments", from.Kind())
			}
		}
		var kind reflect.Type
		if ldType.Func != nil {
			if ldType.Func.Name == "func" || ldType.Func.Name == "void" {
				kind = goType.Out(0)
			} else {
				kind = normal(cgo.Types.LookupKind(ldType.Func.Name))
			}
		}
		call, _, err := asm.UnsafeCall(abi, symbol, send, kind)
		if err != nil {
			return nil, err
		}
		rets := make([]jit.Value, goType.NumOut())
		if ldType.Func != nil {
			into := goType.Out(0)
			if into.Kind() == reflect.Func {
				rets[0] = asm.Go(call[0].UnsafePointer(), func(value unsafe.Pointer) reflect.Value {
					fn, err := compile(name, value, abi, into, *ldType.Func)
					if err != nil {
						panic(err)
					}
					return fn
				})
				return rets, nil
			}
			from := cgo.Types.LookupKind(ldType.Func.Name)
			if from == reflect.Invalid {
				return nil, fmt.Errorf("runtime.link/api/call does not support '%s' results", ldType.Func.Name)
			}
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
					case reflect.Uintptr:
						rets[0] = asm.Convert(call[0], goType.Out(0))
					case reflect.Pointer:
						rets[0] = asm.Convert(call[0].UnsafePointer(), goType.Out(0))
					default:
						return nil, fmt.Errorf("link currently does not support %s -> %s results", ldType.Func.Name, into.Kind())
					}
				}
			}
		}
		return rets, nil
	})
}

// inferValue infers the value of a '-' flagged argument within a tag.
// for example consider the call tag `call:"read func(&char,-int=@1)"`
// the second argument can be inferred from the underlying capacity of
// the Go value passed as the first argument.
func inferValue(asm jit.Assembly, args []jit.Value, rule ffi.Type, kind reflect.Kind, goType reflect.Type) (jit.Value, error) {
	switch {
	case rule.Test.Equality.Check:
		eq := rule.Test.Equality
		switch {
		case eq.Index > 0:
			switch goType.In(int(eq.Index - 1)).Kind() {
			case reflect.Slice:
				return asm.SliceLen(args[eq.Index-1]), nil
			case reflect.String:
				return asm.StringLen(args[eq.Index-1]), nil
			default:
				return jit.Value{}, errors.New("not implemented")
			}
		case eq.Const != "":
			//s := cgo.Constants.Lookup(eq.Const)
			return jit.Value{}, errors.New("not implemented")
		default:
			return jit.Value{}, errors.New("not implemented")
		}
	default:
		return jit.Value{}, errors.New("not implemented")
	}
}
