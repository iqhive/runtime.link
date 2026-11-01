package wasm

import (
	"context"
	"errors"
	"io"
	"reflect"
	"runtime"
	"unsafe"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/wasm/internal/ffi"
)

//go:wasmimport github.com/iqhive/runtime.link dlopen
//go:noescape
func dlopen(library string) uint64

//go:wasmimport github.com/iqhive/runtime.link dlsym
//go:noescape
func dlsym(library uint64, symbol string) ffi.Function

//go:wasmimport github.com/iqhive/runtime.link/ffi func_call
func import_func_call(f ffi.Function, args ffi.Structure) ffi.Structure

//go:wasmimport github.com/iqhive/runtime.link/ffi func_args
func import_func_args(f ffi.Function) ffi.Structure

//go:wasmimport github.com/iqhive/runtime.link/ffi string_new
func import_string_new(data uint64, n uint32) ffi.String

//go:wasmimport github.com/iqhive/runtime.link/ffi string_len
func import_string_len(s ffi.String) uint32

//go:wasmimport github.com/iqhive/runtime.link/ffi string_data
func import_string_data(s ffi.String) ffi.Structure

//go:wasmimport github.com/iqhive/runtime.link/ffi string_free
func import_string_free(s ffi.String)

//go:wasmimport github.com/iqhive/runtime.link/ffi decode_uint8
func import_decode_uint8(s ffi.Structure) uint32

//go:wasmimport github.com/iqhive/runtime.link/ffi decode_string
func import_decode_string(s ffi.Structure) ffi.String

//go:wasmimport github.com/iqhive/runtime.link/ffi decode_bool
func import_decode_bool(s ffi.Structure) uint32

//go:wasmimport github.com/iqhive/runtime.link/ffi decode_int64
func import_decode_int64(s ffi.Structure) int64

//go:wasmimport github.com/iqhive/runtime.link/ffi decode_uint64
func import_decode_uint64(s ffi.Structure) uint64

//go:wasmimport github.com/iqhive/runtime.link/ffi decode_float64
func import_decode_float64(s ffi.Structure) float64

//go:wasmimport github.com/iqhive/runtime.link/ffi encode_bool
func import_encode_bool(s ffi.Structure, b uint32)

//go:wasmimport github.com/iqhive/runtime.link/ffi encode_int64
func import_encode_int64(s ffi.Structure, i int64)

//go:wasmimport github.com/iqhive/runtime.link/ffi encode_uint64
func import_encode_uint64(s ffi.Structure, i uint64)

//go:wasmimport github.com/iqhive/runtime.link/ffi encode_float64
func import_encode_float64(s ffi.Structure, f float64)

//go:wasmimport github.com/iqhive/runtime.link/ffi encode_complex128
func import_encode_complex128(s ffi.Structure, re, im float64)

//go:wasmimport github.com/iqhive/runtime.link/ffi encode_string
func import_encode_string(s ffi.Structure, str ffi.String)

//go:wasmimport github.com/iqhive/runtime.link/ffi struct_free
func import_struct_free(s ffi.Structure)

var parent = ffi.API{
	Function: ffi.Functions{
		Args: import_func_args,
		Call: func(_ context.Context, f ffi.Function, args ffi.Structure) ffi.Structure {
			return import_func_call(f, args)
		},
	},
	String: ffi.Strings{
		New:  newHostString,
		Len:  import_string_len,
		Data: import_string_data,
		Free: import_string_free,
	},
	Structure: ffi.Structures{
		Free: import_struct_free,
		Encode: ffi.Encoding{
			Bool: func(s ffi.Structure, b bool) {
				var slot uint32
				if b {
					slot = 1
				}
				import_encode_bool(s, slot)
			},
			Int64:      import_encode_int64,
			Uint64:     import_encode_uint64,
			Float64:    import_encode_float64,
			Complex128: func(s ffi.Structure, c complex128) { import_encode_complex128(s, real(c), imag(c)) },
			String:     import_encode_string,
		},
		Decode: ffi.Decoding{
			Bool:    func(s ffi.Structure) bool { return import_decode_bool(s) != 0 },
			Int64:   import_decode_int64,
			Uint8:   func(s ffi.Structure) uint8 { return uint8(import_decode_uint8(s)) },
			Uint64:  import_decode_uint64,
			Float64: import_decode_float64,
			String:  import_decode_string,
		},
	},
}

// newHostString hands the host the address of a copy of the text. The host reads
// it from guest memory during the call, so the copy only has to outlive that.
func newHostString(r io.Reader, n uint32) ffi.String {
	data := make([]byte, n)
	if _, err := io.ReadFull(r, data); err != nil {
		panic(err)
	}
	str := import_string_new(uint64(uintptr(unsafe.Pointer(unsafe.SliceData(data)))), n)
	runtime.KeepAlive(data)
	return str
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
		return zero
	}
	for fn := range spec.Iter() {
		pc := dlsym(lib, fn.Name)
		if pc == 0 {
			fn.MakeError(errors.New("dlsym failed"))
			continue
		}
		fn.Make(func(args []reflect.Value) []reflect.Value {
			return parent.Invoke(context.Background(), fn, pc, args)
		})
	}
	return zero
}
