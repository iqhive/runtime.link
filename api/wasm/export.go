package wasm

import (
	"context"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/tetratelabs/wazero"
	wasm_api "github.com/tetratelabs/wazero/api"

	"runtime.link/api"
	"runtime.link/ffi"
)

func goTypeToWasmTypes(values []wasm_api.ValueType, t reflect.Type) []wasm_api.ValueType {
	if t.Size() == 0 {
		return values
	}
	switch t.Kind() {
	case reflect.Bool, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Int8, reflect.Int16, reflect.Int32:
		return append(values, wasm_api.ValueTypeI32)
	case reflect.Uint64, reflect.Int64, reflect.Int, reflect.Uint, reflect.Uintptr:
		return append(values, wasm_api.ValueTypeI64)
	case reflect.String, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func, reflect.Ptr, reflect.UnsafePointer, reflect.Interface:
		return append(values, wasm_api.ValueTypeI64)
	case reflect.Float32:
		return append(values, wasm_api.ValueTypeF32)
	case reflect.Float64:
		return append(values, wasm_api.ValueTypeF64)
	case reflect.Complex64:
		return append(values, wasm_api.ValueTypeF32, wasm_api.ValueTypeF32)
	case reflect.Complex128:
		return append(values, wasm_api.ValueTypeF64, wasm_api.ValueTypeF64)
	case reflect.Struct:
		for i := range t.NumField() {
			values = goTypeToWasmTypes(values, t.Field(i).Type)
		}
		return values
	case reflect.Array:
		for range t.Len() {
			values = goTypeToWasmTypes(values, t.Elem())
		}
		return values
	default:
		panic(fmt.Sprintf("wasm import type not implemented %v", t))
	}
}

func decodeGoValueFromWasmStack(child *ffi.API, stack []uint64, t reflect.Type) (reflect.Value, []uint64) {
	if t.Size() == 0 {
		return reflect.Zero(t), stack
	}
	switch t.Kind() {
	case reflect.Bool:
		return reflect.ValueOf(stack[0] != 0).Convert(t), stack[1:]
	case reflect.Int8:
		u32 := wasm_api.DecodeU32(stack[0])
		i8 := *(*int8)(unsafe.Pointer(&u32))
		return reflect.ValueOf(i8).Convert(t), stack[1:]
	case reflect.Int16:
		u32 := wasm_api.DecodeU32(stack[0])
		i16 := *(*int16)(unsafe.Pointer(&u32))
		return reflect.ValueOf(i16).Convert(t), stack[1:]
	case reflect.Int32:
		return reflect.ValueOf(int32(wasm_api.DecodeI32(stack[0]))).Convert(t), stack[1:]
	case reflect.Int64, reflect.Int:
		u64 := stack[0]
		i64 := *(*int64)(unsafe.Pointer(&u64))
		return reflect.ValueOf(i64).Convert(t), stack[1:]
	case reflect.Uint8:
		return reflect.ValueOf(uint8(wasm_api.DecodeU32(stack[0]))).Convert(t), stack[1:]
	case reflect.Uint16:
		return reflect.ValueOf(uint16(wasm_api.DecodeU32(stack[0]))).Convert(t), stack[1:]
	case reflect.Uint32:
		return reflect.ValueOf(wasm_api.DecodeU32(stack[0])).Convert(t), stack[1:]
	case reflect.Uint64, reflect.Uint, reflect.Uintptr:
		return reflect.ValueOf(stack[0]).Convert(t), stack[1:]
	case reflect.Float32:
		return reflect.ValueOf(wasm_api.DecodeF32(stack[0])).Convert(t), stack[1:]
	case reflect.Float64:
		return reflect.ValueOf(wasm_api.DecodeF64(stack[0])).Convert(t), stack[1:]
	case reflect.Complex64:
		return reflect.ValueOf(complex(wasm_api.DecodeF32(stack[0]), wasm_api.DecodeF32(stack[1]))).Convert(t), stack[2:]
	case reflect.Complex128:
		return reflect.ValueOf(complex(wasm_api.DecodeF64(stack[0]), wasm_api.DecodeF64(stack[1]))).Convert(t), stack[2:]
	case reflect.Struct:
		var v = reflect.New(t).Elem()
		for i := range t.NumField() {
			var value reflect.Value
			value, stack = decodeGoValueFromWasmStack(child, stack, t.Field(i).Type)
			v.Field(i).Set(value)
		}
		return v, stack
	case reflect.Array:
		var v = reflect.New(t).Elem()
		for i := 0; i < t.Len(); i++ {
			var value reflect.Value
			value, stack = decodeGoValueFromWasmStack(child, stack, t.Elem())
			v.Index(i).Set(value)
		}
		return v, stack
	default:
		panic(fmt.Sprintf("wasm export type not implemented %v", t))
	}
}

func encodeGoValueToWasmStack(v reflect.Value, child *ffi.API, stack []uint64) []uint64 {
	switch v.Type().Kind() {
	case reflect.Bool:
		if v.Bool() {
			stack[0] = 1
		} else {
			stack[0] = 0
		}
		return stack[1:]
	case reflect.Int8:
		*(*int8)(unsafe.Pointer(&stack[0])) = int8(v.Int())
		return stack[1:]
	case reflect.Int16:
		*(*int16)(unsafe.Pointer(&stack[0])) = int16(v.Int())
		return stack[1:]
	case reflect.Int32:
		stack[0] = wasm_api.EncodeI32(int32(v.Int()))
		return stack[1:]
	case reflect.Int64, reflect.Int:
		stack[0] = wasm_api.EncodeI64(v.Int())
		return stack[1:]
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		stack[0] = wasm_api.EncodeU32(uint32(v.Uint()))
		return stack[1:]
	case reflect.Uint64, reflect.Uint, reflect.Uintptr:
		stack[0] = v.Uint()
		return stack[1:]
	case reflect.Float32:
		stack[0] = wasm_api.EncodeF32(float32(v.Float()))
		return stack[1:]
	case reflect.Float64:
		stack[0] = wasm_api.EncodeF64(v.Float())
		return stack[1:]
	case reflect.Complex64:
		stack[0] = wasm_api.EncodeF32(float32(real(v.Complex())))
		stack[1] = wasm_api.EncodeF32(float32(imag(v.Complex())))
		return stack[2:]
	case reflect.Complex128:
		stack[0] = wasm_api.EncodeF64(real(v.Complex()))
		stack[1] = wasm_api.EncodeF64(imag(v.Complex()))
		return stack[2:]
	case reflect.Struct:
		for i := range v.NumField() {
			stack = encodeGoValueToWasmStack(v.Field(i), child, stack)
		}
		return stack
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			stack = encodeGoValueToWasmStack(v.Index(i), child, stack)
		}
		return stack
	case reflect.String:
		stack[0] = uint64(ffi.NewString(v.String()))
		return stack[1:]
	default:
		panic(fmt.Sprintf("not implemented %v", v.Type()))
	}
}

func export_api(r wazero.Runtime, child *ffi.API, impl api.WithSpecification) {
	spec := api.StructureOf(impl)
	module := r.NewHostModuleBuilder(spec.Name)
	for fn := range api.StructureOf(spec).Iter() {
		var params []wasm_api.ValueType
		for i := range fn.Type.NumIn() {
			params = goTypeToWasmTypes(params, fn.Type.In(i))
		}
		var results []wasm_api.ValueType
		for i := range fn.Type.NumOut() {
			results = goTypeToWasmTypes(results, fn.Type.Out(i))
		}
		if len(results) > 1 {
			results = []wasm_api.ValueType{wasm_api.ValueTypeI64} // spill to pointer
		}
		if fn.NumIn() == 0 && fn.NumOut() == 0 { // trivial case
			module = module.NewFunctionBuilder().WithGoFunction(wasm_api.GoFunc(func(ctx context.Context, stack []uint64) { fn.Call(ctx, nil) }), params, results).Export(fn.Name)
			continue
		}
		module = module.NewFunctionBuilder().WithGoFunction(wasm_api.GoFunc(func(ctx context.Context, stack []uint64) {
			var args = make([]reflect.Value, fn.Type.NumIn())
			var param = stack
			for i := range fn.Type.NumIn() {
				args[i], param = decodeGoValueFromWasmStack(child, param, fn.Type.In(i))
			}
			outs, err := fn.Call(ctx, args)
			if err != nil {
				panic(err)
			}
			switch max(len(outs), len(results)) {
			case 0:
				return
			case 1:
				encodeGoValueToWasmStack(outs[0], child, stack)
			default:
				panic("multiple return values not supported")
			}
		}), params, results).Export(fn.Name)
	}
	if _, err := module.Instantiate(context.Background()); err != nil {
		panic(err)
	}
}

func dynamic_link(r wazero.Runtime, child *ffi.API, impls []api.WithSpecification) {
	module := r.NewHostModuleBuilder("runtime.link")
	type Function struct {
		Pointer api.Function
		Args    [5][]RegisterMapping
		Results [5][]RegisterMapping
	}
	var i64 = []wasm_api.ValueType{
		wasm_api.ValueTypeI64,
	}
	var i64i32i32 = []wasm_api.ValueType{
		wasm_api.ValueTypeI64,
		wasm_api.ValueTypeI32,
		wasm_api.ValueTypeI32,
	}
	var library_names = make(map[string]uint64)
	var library_functions []map[string]ffi.Function
	for _, impl := range impls {
		spec := api.StructureOf(impl)
		function_names := make(map[string]ffi.Function)
		for fn := range api.StructureOf(spec).Iter() {
			function_names[fn.Name] = ffi.NewFunction(fn.Impl.Interface())
		}
		library_functions = append(library_functions, function_names)
		library_names[spec.Name] = uint64(len(library_functions))
	}
	module = module.NewFunctionBuilder().WithGoModuleFunction(wasm_api.GoModuleFunc(func(ctx context.Context, m wasm_api.Module, stack []uint64) {
		library_str := wasm_api.DecodeU32(stack[0])
		library_len := wasm_api.DecodeU32(stack[1])
		library, ok := m.Memory().Read(library_str, library_len)
		if !ok {
			panic("module string out of bounds")
		}
		stack[0] = library_names[string(library)]
		return
	}),
		[]wasm_api.ValueType{
			wasm_api.ValueTypeI32,
			wasm_api.ValueTypeI32,
		},
		[]wasm_api.ValueType{
			wasm_api.ValueTypeI64,
		},
	).Export("dlopen")
	module = module.NewFunctionBuilder().WithGoModuleFunction(wasm_api.GoModuleFunc(func(ctx context.Context, m wasm_api.Module, stack []uint64) {
		library := stack[0] - 1
		function_str := wasm_api.DecodeU32(stack[1])
		function_len := wasm_api.DecodeU32(stack[2])
		function, ok := m.Memory().Read(function_str, function_len)
		if !ok {
			panic("module string out of bounds")
		}
		stack[0] = uint64(library_functions[library][string(function)])
		return
	}), i64i32i32, i64).Export("dlsym")
	if _, err := module.Instantiate(context.Background()); err != nil {
		panic(err)
	}
}
