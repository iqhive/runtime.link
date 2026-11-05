// Package wasm provides a runtime.link API-based ABI for WebAssembly.
//
// When the hosted API implements [api.WithSource] and the Go parameter
// (or result) count matches the expanded wasm value-type count,
// [api.Function.Args] / [api.Function.Outs] are attached as debug names
// on the host function. Multi-slot Go values that expand to several wasm
// params are left unnamed.
//
// NOTE until Go has support for multiple return values, the runtime.link WASM ABI will always use a
// single return value for all functions.
//
// # The "github.com/iqhive/runtime.link" WASM interface
//
// This module is provided by a runtime.link-aware host and provides the capability to call
// APIs dynamically at runtime, all WASM hosts and modules should implement the following;
//
//	dlopen(module_str, module_len i32) i64
//	dlsym(module i64, symbol_str, symbol_len i32) i64/func
//
//	string -> i64, a handle
//	string_new(data i64, len i32) i64
//	string_len(str i64) i32
//	string_data(str i64) i64
//	string_free(str i64)
//
//	slice -> i64i64i64
//	slice_nil(type i64, arr_head, arr_body, arr_tail i64) i32
//	slice_len(type i64, arr_head, arr_body, arr_tail i64) i32
//	slice_cap(type i64, arr_head, arr_body, arr_tail i64) i32
//	slice_type(any i64) i64
//	slice_data(type i64, arr_head, arr_body, arr_tail i64) i32
//	slice_copy(type i64, dst_head, dst_body, dst_tail i64, src_head, src_body, src_tail i64) i32
//	slice_read(type i64, dst i32, arr_head, arr_body, arr_tail i64, off, max i32) i32
//	slice_index(type i64, arr_head, arr_body, arr_tail i64, idx i32) i64
//	slice_write(type i64, arr_head, arr_body, arr_tail i64, ptr i32, off, max i32) i32
//	slice_free(type i64, arr_head, arr_body, arr_tail i64)
//
//	pointer -> i64
//	pointer_nil(type i64, ptr i64) i32
//	pointer_type(any i64) i64
//	pointer_datatype i64, (ptr i64) i32
//	pointer_read(type i64, dst i32, ptr i64)
//	pointer_write(type i64, ptr i64, val i32)
//	pointer_free(type i64, ptr i64)
//
//	unsafe_pointer -> i64
//	unsafe_pointer_nil(ptr i64) i32
//	unsafe_pointer_data(ptr i64) i32
//	unsafe_pointer_read(dst i32, ptr i64, len i32)
//	unsafe_pointer_write(ptr i64, src i32, len i32)
//	unsafe_pointer_free(ptr i64)
//
//	chan -> i64
//	chan_nil(type i64, chan i64) i32
//	chan_len(type i64, chan i64) i32
//	chan_dir(any i64) i32
//	chan_cap(type i64, chan i64) i32
//	chan_type(any i64) i64
//	chan_iter(type i64, yield i64) i32
//	chan_send(type i64, chan i64, val i32)
//	chan_recv(type i64, dst i32, chan i64) i32
//	chan_close(type i64, chan i64)
//	chan_free(type i64, chan i64)
//
//	func -> i64
//	func_nil(fn i64) i32
//	func_args(fn i64) i32
//	func_outs(fn i64) i32
//	func_type(any i64, inout i32) i64
//	func_call(type i64, fn i64, args i32, rets i32)
//	func_jump(fn i64, stack_ptr, stack_len i32)
//	func_jump0(type i64, fn i64) i64
//	func_jump1(type i64, fn i64, arg i64) i64
//	func_jump2(type i64, fn i64, arg1, arg2 i64) i64
//	func_jump4(type i64, fn i64, arg1, arg2, arg3, arg4 i64) i64
//	func_jump8(type i64, fn i64, arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8 i64) i64
//	func_free(fn i64)
//
//	map -> i64
//	map_nil(keyType, valType i64, map i64) i32
//	map_len(keyType, valType i64, map i64) i32
//	map_key(keyType, valType i64, map i64) i64
//	map_type(any i64) i64
//	map_iter(keyType, valType i64, yield i64) i32
//	map_read(keyType, valType i64, dst i32, map i64, key i32) i32
//	map_write(keyType, valType i64, map i64, key, val i32)
//	map_delete(keyType, valType i64, map i64, key i32)
//	map_clear(keyType, valType i64, map i64)
//	map_free(keyType, valType i64, map i64)
//
//	any -> i64i64
//	any_nil(any_head, any_tail i64) i32
//	any_data(any_head, any_tail i64) i32
//	any_read(dst i32, any_head, any_tail i64)
//	any_type(any_head, any_tail i64) i64/type
//	any_free(any_head, any_tail i64)
//
//	type -> i64
//	type_kind(type i64) i32
//	type_align(type i64) i32
//	type_field_align(type i64) i32
//	type_size(type i64) i32
//	type_name(dst i32, type i64)
//	type_package(dst i32, type i64)
//	type_string(dst i32, type i64)
//	type_len(type i64) i32
//	type_field(type i64, idx i32) i64/field
//	type_free(type i64)
//
//	field -> i64
//	field_name(dst i32, field i64)
//	field_package(dst i32, field i64)
//	field_offset(field i64) i32
//	field_type(field i64) i64
//	field_tag(dst i32, field i64)
//	field_embedded(field i64) i32
//	field_free(field i64)
//
// # Dynamic Calling Convention:
//
// func_args starts a call, the guest sends each argument with encode_*, func_call runs the function
// and the guest reads each result with decode_*. Everything crosses by value, as scalars and strings, depth
// first: structs and arrays field by field or element by element, and a slice as its length (-1 for nil)
// then its elements, one host call each, so what the host does to a slice stays on the host. A trailing error
// result reads as a bool and a message. A leading [context.Context] stays on the host side, where it is
// the context given to [Runner.Run]. Maps, pointers, channels, functions and interfaces can't cross and
// panic in the guest, as do slice results.
//
// # Exports Calling Convention:
//
// Every argument is passed in the least number of uint64 arguments that can completely contain it. uint64s
// are not shared between arguments. A string is a handle from string_new, which the caller frees, and an
// [io.Reader] is an address in guest memory. If the results do not fit in a single uint64 return value, then a
// pointer to the results is passed as the last argument. An error result isn't passed at all: it traps the guest.
package wasm

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/wasm/internal/ffi"
	"github.com/tetratelabs/wazero"
	wasm_api "github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type Runner struct {
	wasi    SystemInterface
	wasm    []byte
	module  wazero.CompiledModule
	apis    []api.WithSpecification
	dirty   bool
	context context.Context
	cancel  func()
	runtime wazero.Runtime
}

type SystemInterface struct {
	Args     []string
	Stdout   io.Writer
	Stderr   io.Writer
	Stdin    io.Reader
	Env      map[string]string
	Memory   int // in bytes
	Root     fs.FS
	Entropy  io.Reader
	Yield    func()
	Sleep    func(time.Duration)
	WallTime func() (int64, int32)
	NanoTime func() int64
}

// Run the web assembly bytes.
func Run(ctx context.Context, file []byte, impls ...api.WithSpecification) error {
	var runner Runner
	return runner.once(ctx, file, impls)
}

func CombinedOutput(ctx context.Context, file []byte, impls ...api.WithSpecification) ([]byte, error) {
	var runner Runner
	var buffer bytes.Buffer
	runner.SetSystemInterface(SystemInterface{
		Stdout: &buffer,
		Stderr: &buffer,
	})
	err := runner.once(ctx, file, impls)
	return buffer.Bytes(), err
}

// once runs file as the Runner's only job, and leaves nothing open behind it.
func (r *Runner) once(ctx context.Context, file []byte, impls []api.WithSpecification) error {
	for _, impl := range impls {
		r.Add(impl)
	}
	r.Set(file)
	return errors.Join(r.Run(ctx), r.Close(ctx))
}

func (r *Runner) Add(impl api.WithSpecification) {
	r.apis = append(r.apis, impl)
	r.dirty = true
}

func (r *Runner) SetSystemInterface(si SystemInterface) {
	r.wasi = si
	r.dirty = true
}

func (r *Runner) Set(wasm []byte) {
	r.wasm = wasm
	r.module = nil
}

func (r *Runner) Compile(ctx context.Context) (err error) {
	if err := r.assertRuntime(); err != nil {
		return err
	}
	r.module, err = r.runtime.CompileModule(ctx, r.wasm)
	return
}

// Close releases the runtime, and with it everything compiled or linked in it.
// Run or Compile start a new one, so a Runner stays usable afterwards.
func (r *Runner) Close(ctx context.Context) error {
	if r.runtime == nil {
		return nil
	}
	err := r.runtime.Close(ctx)
	r.cancel()
	r.runtime, r.module = nil, nil // compiled code doesn't outlive its runtime
	return err
}

func (r *Runner) assertRuntime() error {
	if r.dirty || r.runtime == nil {
		if err := r.Close(context.Background()); err != nil {
			return err
		}
		r.context, r.cancel = context.WithCancel(context.Background())
		config := wazero.NewRuntimeConfigCompiler()
		if r.wasi.Memory > 0 {
			config = config.WithMemoryLimitPages(uint32(r.wasi.Memory / 65536))
		}
		r.runtime = wazero.NewRuntimeWithConfig(r.context, config)
		wasi_snapshot_preview1.MustInstantiate(r.context, r.runtime)
		// Host modules live as long as the runtime and a name can only be
		// instantiated once, so these can't wait for each Run.
		var child = ffi.New()
		import_api(r.runtime, &child, &child)
		export_api(r.runtime, &child, child)
		for _, impl := range r.apis {
			export_api(r.runtime, &child, impl)
		}
		dynamic_link(r.runtime, &child, append(r.apis, child))
		r.dirty = false
	}
	return nil
}

func (r *Runner) Run(ctx context.Context) error {
	if err := r.assertRuntime(); err != nil {
		return err
	}
	config := wazero.NewModuleConfig()
	if r.wasi.Stdout != nil {
		config = config.WithStdout(r.wasi.Stdout)
	}
	if r.wasi.Stderr != nil {
		config = config.WithStderr(r.wasi.Stderr)
	}
	if r.wasi.Stdin != nil {
		config = config.WithStdin(r.wasi.Stdin)
	}
	if r.wasi.Args != nil {
		config = config.WithArgs(r.wasi.Args...)
	} else {
		config = config.WithArgs("")
	}
	for k, v := range r.wasi.Env {
		config = config.WithEnv(k, v)
	}
	if r.wasi.Root != nil {
		config = config.WithFS(r.wasi.Root)
	}
	if r.wasi.Entropy != nil {
		config = config.WithRandSource(r.wasi.Entropy)
	}
	if r.wasi.Yield != nil {
		config = config.WithOsyield(r.wasi.Yield)
	}
	if r.wasi.Sleep != nil {
		config = config.WithNanosleep(func(ns int64) {
			r.wasi.Sleep(time.Duration(ns))
		})
	}
	if r.wasi.WallTime != nil {
		config = config.WithWalltime(r.wasi.WallTime, 1)
	}
	if r.wasi.NanoTime != nil {
		config = config.WithNanotime(r.wasi.NanoTime, 1)
	}
	guest, err := r.instantiate(ctx, config)
	if err != nil {
		return err
	}
	return guest.Close(ctx)
}

func (r *Runner) instantiate(ctx context.Context, config wazero.ModuleConfig) (wasm_api.Module, error) {
	if r.module == nil {
		return r.runtime.InstantiateWithConfig(ctx, r.wasm, config)
	}
	return r.runtime.InstantiateModule(ctx, r.module, config)
}
