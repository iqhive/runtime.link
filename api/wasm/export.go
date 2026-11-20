package wasm

import (
	"context"
	"fmt"
	"io"
	"math"
	"reflect"
	"slices"
	"sync"
	"unsafe"

	"github.com/tetratelabs/wazero"
	wasm_api "github.com/tetratelabs/wazero/api"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/wasm/internal/ffi"
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

func decodeGoValueFromWasmStack(child *ffi.API, memory wasm_api.Memory, stack []uint64, t reflect.Type) (reflect.Value, []uint64) {
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
	case reflect.String:
		return reflect.ValueOf(child.GoString(ffi.String(stack[0]))).Convert(t), stack[1:]
	case reflect.Interface:
		if t != reflect.TypeFor[io.Reader]() {
			panic(fmt.Sprintf("wasm export type not implemented %v", t))
		}
		return reflect.ValueOf(&guestReader{memory: memory, offset: stack[0]}).Convert(t), stack[1:]
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
			value, stack = decodeGoValueFromWasmStack(child, memory, stack, t.Field(i).Type)
			v.Field(i).Set(value)
		}
		return v, stack
	case reflect.Array:
		var v = reflect.New(t).Elem()
		for i := 0; i < t.Len(); i++ {
			var value reflect.Value
			value, stack = decodeGoValueFromWasmStack(child, memory, stack, t.Elem())
			v.Index(i).Set(value)
		}
		return v, stack
	default:
		panic(fmt.Sprintf("wasm export type not implemented %v", t))
	}
}

// guestReader is what an io.Reader parameter becomes on the host: the guest
// passes an address, and the host reads the guest's memory from there.
type guestReader struct {
	memory wasm_api.Memory
	offset uint64
}

func (r *guestReader) Read(p []byte) (int, error) {
	if r.offset > math.MaxUint32 {
		return 0, fmt.Errorf("wasm: guest address %#x doesn't fit in 32 bits", r.offset)
	}
	data, ok := r.memory.Read(uint32(r.offset), uint32(len(p)))
	if !ok {
		return 0, fmt.Errorf("wasm: reading %d bytes at guest address %#x is out of bounds", len(p), r.offset)
	}
	r.offset += uint64(len(p))
	return copy(p, data), nil
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
	case reflect.Int8, reflect.Int16, reflect.Int32:
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
		stack[0] = uint64(child.NewString(v.String()))
		return stack[1:]
	default:
		panic(fmt.Sprintf("not implemented %v", v.Type()))
	}
}

// guest is what the host keeps for one guest: the bridge that holds its
// handles, and the handles that dlsym has given it. Each [Runner.Run] makes
// its own, so a guest can neither use nor free what another guest holds, and
// whatever it leaves behind goes with it.
type guest struct {
	bridge ffi.API

	reflected sync.Once
	functions map[string]api.Function // of the bridge, by name

	mu     sync.Mutex
	linked map[symbol]ffi.Function
}

// symbol is a function as dlsym names it: the handle of its library, from
// dlopen, and its name there.
type symbol struct {
	library uint64
	name    string
}

func newGuest() *guest {
	return &guest{bridge: ffi.New(), linked: make(map[symbol]ffi.Function)}
}

// function returns the bridge function of the given name on this guest's own
// bridge. The host modules are instantiated once for the runtime, so each call
// of a bridge function is passed on to the bridge of the guest that made it.
func (g *guest) function(name string) api.Function {
	g.reflected.Do(func() {
		g.functions = make(map[string]api.Function)
		for fn := range api.StructureOf(&g.bridge).Iter() {
			g.functions[fn.Name] = fn
		}
	})
	return g.functions[name]
}

// link returns this guest's handle to a function that dlsym found. It is the
// same handle every time, so a guest that looks a function up over and over
// holds no more of the host's memory for it.
func (g *guest) link(name symbol, fn api.Function, bridged bool) ffi.Function {
	g.mu.Lock()
	defer g.mu.Unlock()
	if handle, ok := g.linked[name]; ok {
		return handle
	}
	if bridged {
		fn = g.function(fn.Name)
	}
	handle := g.bridge.NewFunction(fn.Impl.Interface())
	g.linked[name] = handle
	return handle
}

type guestKey struct{}

// withGuest returns the context that a guest runs with. It is how a host
// function, which wazero gives the context of the call and little else, knows
// which guest is calling.
func withGuest(ctx context.Context, g *guest) context.Context {
	return context.WithValue(ctx, guestKey{}, g)
}

// guestOf returns the guest that a call belongs to. A call that [Runner.Run]
// didn't start belongs to none, and uses what the runtime keeps for those.
func guestOf(ctx context.Context, outside *guest) *guest {
	if g, ok := ctx.Value(guestKey{}).(*guest); ok {
		return g
	}
	return outside
}

// isBridge reports whether impl is the bridge, whose functions work on the
// handles of the guest that calls them.
func isBridge(impl api.WithSpecification) bool {
	_, ok := impl.(ffi.API)
	return ok
}

func export_api(r wazero.Runtime, outside *guest, impl api.WithSpecification) {
	spec := api.StructureOf(impl)
	bridged := isBridge(impl)
	// called returns fn as a guest calls it: a function of the bridge is the
	// one on the guest's own bridge.
	called := func(g *guest, fn api.Function) api.Function {
		if bridged {
			return g.function(fn.Name)
		}
		return fn
	}
	module := r.NewHostModuleBuilder(spec.Name)
	for fn := range api.StructureOf(spec).Iter() {
		// A context.Context comes from the host and an error traps the guest,
		// so neither has a place in the wasm signature.
		var params []wasm_api.ValueType
		for i := range fn.NumIn() {
			params = goTypeToWasmTypes(params, fn.In(i))
		}
		var results []wasm_api.ValueType
		for i := range fn.NumOut() {
			results = goTypeToWasmTypes(results, fn.Type.Out(i))
		}
		// Results that take more than one uint64 can't be returned, so the
		// guest passes the address to write them to after its arguments.
		spilled := 0
		if len(results) > 1 {
			spilled = len(results)
			params = append(params, wasm_api.ValueTypeI64)
			results = nil
		}
		if fn.NumIn() == 0 && fn.NumOut() == 0 { // trivial case
			module = module.NewFunctionBuilder().WithGoFunction(wasm_api.GoFunc(func(ctx context.Context, stack []uint64) {
				if _, err := called(guestOf(ctx, outside), fn).Call(ctx, nil); err != nil {
					panic(err)
				}
			}), params, results).Export(fn.Name)
			continue
		}
		module = applyWasmNames(module.NewFunctionBuilder(), fn, params, results, spilled > 0).WithGoModuleFunction(wasm_api.GoModuleFunc(func(ctx context.Context, m wasm_api.Module, stack []uint64) {
			caller := guestOf(ctx, outside)
			child := &caller.bridge
			var args = make([]reflect.Value, fn.NumIn())
			var param = stack
			for i := range fn.NumIn() {
				args[i], param = decodeGoValueFromWasmStack(child, m.Memory(), param, fn.In(i))
			}
			// What is left of the stack after the arguments is the address for
			// spilled results. It is checked before the call, so a function
			// whose results can't be delivered doesn't run.
			err := spilledResultsFit(m.Memory(), param, spilled)
			var outs []reflect.Value
			if err == nil {
				outs, err = called(caller, fn).Call(ctx, args)
			}
			if err != nil {
				panic(err)
			}
			if spilled == 0 {
				encodeResults(outs, child, stack)
				return
			}
			slots := make([]uint64, spilled)
			encodeResults(outs, child, slots)
			if err := writeSpilledResults(m.Memory(), param[0], slots); err != nil {
				panic(err)
			}
		}), params, results).Export(fn.Name)
	}
	if _, err := module.Instantiate(context.Background()); err != nil {
		panic(err)
	}
}

func encodeResults(outs []reflect.Value, child *ffi.API, slots []uint64) {
	for _, out := range outs {
		slots = encodeGoValueToWasmStack(out, child, slots)
	}
}

// bytesPerSlot is the size in the guest's memory of each uint64 of a result.
const bytesPerSlot = 8

// spilledResultsFit returns an error if count uint64s of results don't fit in
// the guest's memory at the address it passed, which is first in address. No
// results are spilled when count is 0.
func spilledResultsFit(memory wasm_api.Memory, address []uint64, count int) error {
	if count == 0 {
		return nil
	}
	if address[0] > math.MaxUint32 {
		return fmt.Errorf("wasm: guest address %#x for the results doesn't fit in 32 bits", address[0])
	}
	if _, ok := memory.Read(uint32(address[0]), uint32(count)*bytesPerSlot); !ok {
		return fmt.Errorf("wasm: %d results of 8 bytes at guest address %#x are out of bounds", count, address[0])
	}
	return nil
}

// writeSpilledResults writes the uint64s of the results one after another,
// little-endian, which is how the guest reads them whatever they hold.
func writeSpilledResults(memory wasm_api.Memory, address uint64, slots []uint64) error {
	for i, slot := range slots {
		if !memory.WriteUint64Le(uint32(address)+uint32(i)*bytesPerSlot, slot) {
			return fmt.Errorf("wasm: %d results of 8 bytes at guest address %#x are out of bounds", len(slots), address)
		}
	}
	return nil
}

func applyWasmNames(b wazero.HostFunctionBuilder, fn api.Function, params, results []wasm_api.ValueType, spilled bool) wazero.HostFunctionBuilder {
	names := fn.Args
	if spilled {
		// The last parameter is the address for the results, not an argument.
		names = append(slices.Clone(names), "results")
	}
	if n := len(fn.Args); n > 0 && len(names) == len(params) && everyArgFillsOneSlot(fn) {
		b = b.WithParameterNames(names...)
	}
	if n := len(fn.Outs); n > 0 && n == len(results) && !slices.Contains(fn.Outs, "") {
		b = b.WithResultNames(fn.Outs...)
	}
	return b
}

// everyArgFillsOneSlot reports whether fn's parameter names line up with its
// wasm params. Equal counts don't show that: an empty struct has no slot and a
// complex number has two. An unknown name is "", and wazero would keep that.
func everyArgFillsOneSlot(fn api.Function) bool {
	for i := range fn.NumIn() {
		if len(goTypeToWasmTypes(nil, fn.In(i))) != 1 {
			return false
		}
	}
	return fn.Named()
}

func dynamic_link(r wazero.Runtime, outside *guest, impls []api.WithSpecification) {
	module := r.NewHostModuleBuilder("github.com/iqhive/runtime.link")
	type Function struct {
		Pointer api.Function
		Args    [5][]RegisterMapping
		Results [5][]RegisterMapping
	}
	type library struct {
		functions map[string]api.Function
		bridged   bool
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
	var libraries []library
	for _, impl := range impls {
		spec := api.StructureOf(impl)
		function_names := make(map[string]api.Function)
		for fn := range api.StructureOf(spec).Iter() {
			function_names[fn.Name] = fn
		}
		libraries = append(libraries, library{functions: function_names, bridged: isBridge(impl)})
		library_names[spec.Name] = uint64(len(libraries))
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
		// 0 and anything past the libraries read as no such function, as
		// dlopen answers 0 for no such library: a guest that never opened
		// one is not crashed for asking.
		if stack[0] == 0 || stack[0] > uint64(len(libraries)) {
			stack[0] = 0
			return
		}
		library := libraries[stack[0]-1]
		function_str := wasm_api.DecodeU32(stack[1])
		function_len := wasm_api.DecodeU32(stack[2])
		function, ok := m.Memory().Read(function_str, function_len)
		if !ok {
			panic("module string out of bounds")
		}
		name := symbol{library: stack[0], name: string(function)}
		fn, ok := library.functions[name.name]
		if !ok {
			stack[0] = 0 // which a guest reads as no such function
			return
		}
		// The handle is the calling guest's own, and to that guest's own
		// bridge if the function is one of the bridge's.
		stack[0] = uint64(guestOf(ctx, outside).link(name, fn, library.bridged))
		return
	}), i64i32i32, i64).Export("dlsym")
	if _, err := module.Instantiate(context.Background()); err != nil {
		panic(err)
	}
}
