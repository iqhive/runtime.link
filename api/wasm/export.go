package wasm

import (
	"context"
	"encoding/binary"
	"reflect"
	"unsafe"

	"github.com/tetratelabs/wazero"
	wasm_api "github.com/tetratelabs/wazero/api"

	"runtime.link/api"
)

func export(r wazero.Runtime, impl api.WithSpecification) {
	spec := api.StructureOf(impl)
	module := r.NewHostModuleBuilder(spec.Name)
	for fn := range api.StructureOf(spec).Iter() {
		var params []wasm_api.ValueType
		var results []wasm_api.ValueType
		module = module.NewFunctionBuilder().WithGoFunction(wasm_api.GoFunc(func(ctx context.Context, stack []uint64) {
			panic("not implemented")
		}), params, results).Export(fn.Name)
	}
	if _, err := module.Instantiate(context.Background()); err != nil {
		panic(err)
	}
}

type wrappedPointers struct {
	strings []string
}

func (w *wrappedPointers) dynamic_call(ctx context.Context, fn api.Function, reads func(uint32, uint32) []byte, write func(uint32, []byte)) {
	var offs int
	var args = make([]reflect.Value, fn.Type.NumIn())
	for i := 0; i < fn.Type.NumIn(); i++ {
		in := fn.Type.In(i)
		args[i] = reflect.New(in).Elem()
		switch in.Kind() {
		default:
			panic("not implemented")
		}
		offs += int(fn.Type.In(i).Size())
	}
	outs, err := fn.Call(ctx, args)
	if err != nil {
		panic(err)
	}
	var write_head uint32
	for i := 0; i < fn.Type.NumOut(); i++ {
		out := fn.Type.Out(i)
		switch out.Kind() {
		case reflect.String:
			s := outs[i].String()
			w.strings = append(w.strings, s)
			var wasm_string [16]byte
			binary.LittleEndian.PutUint64(wasm_string[:], uint64(len(w.strings)))
			binary.LittleEndian.PutUint64(wasm_string[8:], uint64(len(s)))
			write(write_head, wasm_string[:])
		default:
			panic("not implemented")
		}
		offs += int(out.Size())
	}
}

func dynamic_link(r wazero.Runtime, impls []api.WithSpecification) {
	var wrapped wrappedPointers
	module := r.NewHostModuleBuilder("runtime.link")
	type Function struct {
		Pointer api.Function
		Args    [5][]RegisterMapping
		Results [5][]RegisterMapping
	}
	var i64 = []wasm_api.ValueType{
		wasm_api.ValueTypeI64,
	}
	var i32 = []wasm_api.ValueType{
		wasm_api.ValueTypeI32,
	}
	var i64i64 = []wasm_api.ValueType{
		wasm_api.ValueTypeI64,
		wasm_api.ValueTypeI64,
	}
	var i64i32i32 = []wasm_api.ValueType{
		wasm_api.ValueTypeI64,
		wasm_api.ValueTypeI32,
		wasm_api.ValueTypeI32,
	}
	var i32i64i64i32i32 = []wasm_api.ValueType{
		wasm_api.ValueTypeI32,
		wasm_api.ValueTypeI64,
		wasm_api.ValueTypeI64,
		wasm_api.ValueTypeI32,
		wasm_api.ValueTypeI32,
	}
	var library_names = make(map[string]uint64)
	var library_functions []map[string]uint64
	var functions []Function
	for _, impl := range impls {
		spec := api.StructureOf(impl)
		function_names := make(map[string]uint64)
		for fn := range api.StructureOf(spec).Iter() {
			args0, rets0, _, _ := squeezeRegistersFor(fn.Impl.Type(), 0)
			args1, rets1, _, _ := squeezeRegistersFor(fn.Impl.Type(), 1)
			args2, rets2, _, _ := squeezeRegistersFor(fn.Impl.Type(), 2)
			args4, rets4, _, _ := squeezeRegistersFor(fn.Impl.Type(), 4)
			args8, rets8, _, _ := squeezeRegistersFor(fn.Impl.Type(), 8)
			functions = append(functions, Function{
				Pointer: fn,
				Args: [5][]RegisterMapping{
					args0, args1, args2, args4, args8,
				},
				Results: [5][]RegisterMapping{
					rets0, rets1, rets2, rets4, rets8,
				},
			})
			function_names[fn.Name] = uint64(len(functions))
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
		stack[0] = library_functions[library][string(function)]
		return
	}), i64i32i32, i64).Export("dlsym")

	module = module.NewFunctionBuilder().WithGoModuleFunction(wasm_api.GoModuleFunc(func(ctx context.Context, m wasm_api.Module, stack []uint64) {
		stack[0] = wasm_api.EncodeU32(uint32(stack[1]))
	}), i64i64, i32).Export("string_len")

	module = module.NewFunctionBuilder().WithGoModuleFunction(wasm_api.GoModuleFunc(func(ctx context.Context, m wasm_api.Module, stack []uint64) {
		dst := wasm_api.DecodeU32(stack[0])
		str := stack[1]
		off := stack[3]
		max := stack[4]
		m.Memory().WriteString(dst, wrapped.strings[str-1][off:off+max])
	}), i32i64i64i32i32, i32).Export("string_copy")

	i64s := []wasm_api.ValueType{
		wasm_api.ValueTypeI64,
		wasm_api.ValueTypeI64,
		wasm_api.ValueTypeI64,
		wasm_api.ValueTypeI64,
		wasm_api.ValueTypeI64,
		wasm_api.ValueTypeI64,
		wasm_api.ValueTypeI64,
		wasm_api.ValueTypeI64,
		wasm_api.ValueTypeI64,
	}
	module = module.NewFunctionBuilder().WithGoModuleFunction(wasm_api.GoModuleFunc(func(ctx context.Context, m wasm_api.Module, stack []uint64) {
		fn := functions[stack[0]-1]
		stack_ptr := wasm_api.DecodeU32(stack[1])
		stack_len := wasm_api.DecodeU32(stack[2])
		reads := func(offset, length uint32) []byte {
			if offset+length > stack_len {
				panic("stack out of bounds")
			}
			b, ok := m.Memory().Read(stack_ptr+offset, length)
			if !ok {
				panic("stack out of bounds")
			}
			return b
		}
		write := func(offset uint32, b []byte) {
			if offset+uint32(len(b)) > stack_len {
				panic("stack out of bounds")
			}
			if !m.Memory().Write(stack_ptr+offset, b) {
				panic("stack out of bounds")
			}
		}
		wrapped.dynamic_call(ctx, fn.Pointer, reads, write)
	}), i64i32i32, nil).Export("func_jump")
	direct := wasm_api.GoModuleFunc(func(ctx context.Context, m wasm_api.Module, stack []uint64) {
		fn := functions[stack[0]-1]
		reads := func(offset, length uint32) []byte {
			return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(stack))), unsafe.Sizeof(uint64(0))*uintptr(len(stack)))[offset : offset+length]
		}
		write := func(offset uint32, b []byte) {
			copy(unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(stack))), unsafe.Sizeof(uint64(0))*uintptr(len(stack)))[offset:], b)
		}
		wrapped.dynamic_call(ctx, fn.Pointer, reads, write)
	})
	module = module.NewFunctionBuilder().WithGoModuleFunction(direct, i64s[:1], i64).Export("func_jump0")
	module = module.NewFunctionBuilder().WithGoModuleFunction(direct, i64s[:2], i64).Export("func_jump1")
	module = module.NewFunctionBuilder().WithGoModuleFunction(direct, i64s[:3], i64).Export("func_jump2")
	module = module.NewFunctionBuilder().WithGoModuleFunction(direct, i64s[:5], i64).Export("func_jump4")
	module = module.NewFunctionBuilder().WithGoModuleFunction(direct, i64s, i64).Export("func_jump8")
	if _, err := module.Instantiate(context.Background()); err != nil {
		panic(err)
	}
}
