package wasm

import (
	"errors"
	"reflect"

	"runtime.link/api"
	"runtime.link/ffi"
)

//go:wasmimport runtime.link dlopen
//go:noescape
func dlopen(library string) uint64

//go:wasmimport runtime.link dlsym
//go:noescape
func dlsym(library uint64, symbol string) ffi.Function

//go:wasmimport runtime.link/ffi func_call
func import_func_call(f ffi.Function, args ffi.Structure) ffi.Structure

//go:wasmimport runtime.link/ffi func_args
func import_func_args(f ffi.Function) ffi.Structure

//go:wasmimport runtime.link/ffi string_new
func import_string_new(r ffi.Type, n uint32) ffi.String

//go:wasmimport runtime.link/ffi string_len
func import_string_len(s ffi.String) uint32

//go:wasmimport runtime.link/ffi string_data
func import_string_data(s ffi.String) ffi.Structure

//go:wasmimport runtime.link/ffi string_free
func import_string_free(s ffi.String)

//go:wasmimport runtime.link/ffi decode_uint8
func import_decode_uint8(s ffi.Structure) uint32

//go:wasmimport runtime.link/ffi decode_string
func import_decode_string(s ffi.Structure) ffi.String

//go:wasmimport runtime.link/ffi struct_free
func import_struct_free(s ffi.Structure)

var parent = ffi.API{
	Function: ffi.Functions{
		Args: import_func_args,
		Call: import_func_call,
	},
	String: ffi.Strings{
		Len:  import_string_len,
		Data: import_string_data,
		Free: import_string_free,
	},
	Structure: ffi.Structures{
		Free: import_struct_free,
		Decode: ffi.Decoding{
			Uint8:  func(s ffi.Structure) uint8 { return uint8(import_decode_uint8(s)) },
			String: import_decode_string,
		},
	},
}

var local = ffi.New()

func Import[API api.WithSpecification]() API {
	var zero API
	spec := api.StructureOf(&zero)
	lib := dlopen(spec.Name)
	if lib == 0 {
		for fn := range spec.Iter() {
			fn.MakeError(errors.New("dlopen failed"))
		}
	}
	for fn := range spec.Iter() {
		pc := dlsym(lib, fn.Name)
		if fn.Type.NumIn() == 0 && fn.Type.NumOut() == 0 {
			fn.Make(func(args []reflect.Value) []reflect.Value {
				parent.Function.Call(pc, 0)
				return nil
			})
			continue
		}
		num_out := fn.Type.NumOut()
		fn.Make(func(args []reflect.Value) []reflect.Value {
			pargs := parent.Function.Args(pc)
			for _, arg := range args {
				parent.Encode(pargs, arg)
			}
			pouts := parent.Function.Call(pc, pargs)
			switch num_out {
			case 0:
				return nil
			case 1:
				var results []reflect.Value
				var result = reflect.New(fn.Type.Out(0))
				parent.Decode(result.Interface(), pouts)
				results = append(results, result.Elem())
				return results
			}
			panic("multiple return values not supported")
		})
	}
	return zero
}
