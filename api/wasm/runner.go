// Package wasm provides a runtime.link API-based ABI for WebAssembly.
//
// When the hosted API implements [api.WithSource] and the Go parameter
// (or result) count matches the expanded wasm value-type count,
// [api.Function.Args] / [api.Function.Outs] are attached as debug names
// on the host function. Multi-slot Go values that expand to several wasm
// params are left unnamed. The address that a guest passes for results of more
// than one uint64 is named "results".
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
// slice_make, chan_make and pointer_new trap when what they would make takes more than
// 16MiB of host memory, as the guest names both the type and how many of it.
//
// # Dynamic Calling Convention:
//
// func_args starts a call, the guest sends each argument with encode_*, func_call runs the function
// and the guest reads each result with decode_*. Everything crosses by value, as scalars and strings, depth
// first: structs and arrays field by field or element by element, and a slice as its length (-1 for nil)
// then its elements, one host call each, so what the host does to a slice stays on the host. A trailing error
// result reads as a bool and a message. A leading [context.Context] stays on the host side, where it is
// the context of the guest's [Runner.Run]: the one given to it, which also ends when the Run does. Maps,
// pointers, channels, functions and interfaces can't cross and panic in the guest, as do slice results. The
// host calls no function that has such a result, as a guest would get a handle to the host's own value:
// func_call traps instead.
//
// A handle belongs to the guest that was given it. Each [Runner.Run] has its own, which go when its guest
// ends, and the host checks what a handle is of before using it. func_call may itself be the function that
// func_call calls, but only 16 calls deep.
//
// # Exports Calling Convention:
//
// Every argument is passed in the least number of uint64 arguments that can completely contain it. uint64s
// are not shared between arguments. A string is a handle from string_new, which the caller frees, and an
// [io.Reader] is an address in guest memory. If the results do not fit in a single uint64 return value, then a
// pointer to the results is passed as the last argument: an i64 address in guest memory, where the host writes
// each uint64 of the results in turn, little-endian, laid out as they would be as arguments, and the function
// returns nothing. An error result isn't passed at all: it traps the guest.
package wasm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"sync"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type Runner struct {
	// mu guards every field, so that Close can stop a Run that another
	// goroutine is in. Run holds it neither while it compiles its guest nor
	// while the guest runs.
	mu      sync.Mutex
	wasi    SystemInterface
	wasm    []byte
	module  *compiled   // the code of wasm, once a Run or Compile has asked for it
	retired []*compiled // code that Set replaced, until no Run is using it
	apis    []api.WithSpecification
	dirty   bool
	context context.Context // ends when the runtime is closed
	cancel  func()
	runtime wazero.Runtime
	users   *users // the calls of Run and Compile that are using the runtime
	outside *guest // holds the handles of calls that no Run started
}

// compiled is the compiled code of one guest in one runtime. wazero keeps a
// single copy of the code of a binary however often it is compiled, and gives
// it up when any one of those is closed, so a Runner compiles each binary once,
// for every Run of it, and closes it once: when Set has replaced it and no Run
// is using it any more.
type compiled struct {
	id      [sha256.Size]byte // of the binary
	compile sync.Once
	module  wazero.CompiledModule
	err     error
	runs    int // the calls that are using it, guarded by Runner.mu
}

// ready returns the compiled code, compiling it for the first caller while
// any other waits for that.
func (c *compiled) ready(ctx context.Context, runtime wazero.Runtime, wasm []byte) (wazero.CompiledModule, error) {
	c.compile.Do(func() {
		c.err = errors.New("wasm: the guest could not be compiled")
		c.module, c.err = runtime.CompileModule(ctx, wasm)
	})
	return c.module, c.err
}

// users counts the calls of Run and Compile that are using a runtime. Closing
// a runtime releases the files and the compiled code of guests that are still
// running in it, and of one that is still being compiled, so a runtime that is
// closed while it has users is left for the last of them to close.
type users struct {
	mu      sync.Mutex
	count   int
	closing wazero.Runtime
}

// enter adds a user, and reports whether it is the only one.
func (u *users) enter() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.count++
	return u.count == 1
}

// leave takes a user off, and closes the runtime if it was left to this one.
func (u *users) leave(ctx context.Context) error {
	u.mu.Lock()
	u.count--
	if u.count > 0 || u.closing == nil {
		u.mu.Unlock()
		return nil
	}
	runtime := u.closing
	u.closing = nil
	u.mu.Unlock()
	return runtime.Close(ctx)
}

// close closes the runtime now if it has no users, and leaves it to the last
// of them if it has. No user enters after it: the Runner has let go of the
// runtime by then.
func (u *users) close(ctx context.Context, runtime wazero.Runtime) error {
	u.mu.Lock()
	if u.count > 0 {
		u.closing = runtime
		u.mu.Unlock()
		return nil
	}
	u.mu.Unlock()
	return runtime.Close(ctx)
}

type SystemInterface struct {
	Args     []string
	Stdout   io.Writer
	Stderr   io.Writer
	Stdin    io.Reader
	Env      map[string]string
	Memory   int // limit in bytes, rounded down to 64KiB pages; above 4GiB it is 4GiB, all a guest can address; 0 is a default of 64MiB
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
	r.mu.Lock()
	defer r.mu.Unlock()
	r.apis = append(r.apis, impl)
	r.dirty = true
}

func (r *Runner) SetSystemInterface(si SystemInterface) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.wasi = si
	r.dirty = true
}

func (r *Runner) Set(wasm []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.wasm = wasm
	if r.module != nil {
		r.retired = append(r.retired, r.module)
		r.module = nil
	}
}

// Compile compiles the guest ahead of its first Run. Every Run of a guest
// uses the same compiled code, which Run compiles itself if Compile has not.
func (r *Runner) Compile(ctx context.Context) (err error) {
	begun, err := r.begin(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.end(ctx, begun)) }()
	_, err = begun.code.ready(ctx, begun.runtime, begun.wasm)
	return err
}

// Close releases the runtime, and with it everything compiled or linked in it.
// Run or Compile start a new one, so a Runner stays usable afterwards. A Run
// that another goroutine is in has its guest stopped, and returns an error:
// Close does not wait for it, and the runtime is released when it has returned.
func (r *Runner) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.close(ctx)
}

// close is Close for a caller that holds r.mu.
func (r *Runner) close(ctx context.Context) error {
	if r.runtime == nil {
		return nil
	}
	// Cancelled first, as closing the runtime doesn't interrupt a guest.
	r.cancel()
	runtime, users := r.runtime, r.users
	// Compiled code doesn't outlive its runtime.
	r.runtime, r.users, r.module, r.retired = nil, nil, nil, nil
	return users.close(ctx, runtime)
}

// assertRuntime is called with r.mu held.
func (r *Runner) assertRuntime(ctx context.Context) error {
	if !r.dirty && r.runtime != nil {
		return nil
	}
	if err := r.close(ctx); err != nil {
		return err
	}
	r.context, r.cancel = context.WithCancel(context.Background())
	// Without this wazero runs a guest to its end whatever happens to the
	// context of the call, and a guest that loops has no end.
	config := wazero.NewRuntimeConfigCompiler().WithCloseOnContextDone(true)
	if r.wasi.Memory > 0 {
		config = config.WithMemoryLimitPages(memoryLimitPages(r.wasi.Memory))
	} else {
		config = config.WithMemoryLimitPages(memoryLimitPages(defaultMemory))
	}
	r.runtime = wazero.NewRuntimeWithConfig(r.context, config)
	r.users = new(users)
	r.outside = newGuest()
	if err := r.link(); err != nil {
		// Nothing is using a runtime that was never handed out. The Runner
		// stays dirty, so the next call meets the same mistake.
		return errors.Join(err, r.close(ctx))
	}
	r.dirty = false
	return nil
}

// link instantiates the host modules of a new runtime. They live as long as
// the runtime and a name can only be instantiated once, so they can't wait for
// each Run. They find the guest that is calling in the context of the call,
// see [guestOf]. It is called with r.mu held.
func (r *Runner) link() error {
	if _, err := wasi_snapshot_preview1.Instantiate(r.context, r.runtime); err != nil {
		return err
	}
	child := r.outside.bridge
	import_api(r.runtime, &child, &child)
	if err := export_api(r.runtime, r.outside, child); err != nil {
		return err
	}
	for _, impl := range r.apis {
		if err := export_api(r.runtime, r.outside, impl); err != nil {
			return err
		}
	}
	return dynamic_link(r.runtime, r.outside, append(r.apis, child))
}

// use is what a call of Run or Compile takes from the Runner while it holds
// the lock, to go on with after it has let go of it.
type use struct {
	runtime wazero.Runtime
	open    context.Context // ends when the runtime is closed
	users   *users
	code    *compiled
	wasm    []byte
	wasi    SystemInterface
	alone   bool // no other call was using the runtime when this one began
}

// begin makes the runtime and the entry for the guest's code, and counts the
// caller as a user of both until [Runner.end].
func (r *Runner) begin(ctx context.Context) (use, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.assertRuntime(ctx); err != nil {
		return use{}, err
	}
	if r.module == nil {
		r.module = r.code()
		// Whatever Set replaced and no call is using is closed here, where
		// the code that is used has just been taken out of it.
		if err := r.release(ctx); err != nil {
			return use{}, err
		}
	}
	r.module.runs++
	return use{
		runtime: r.runtime,
		open:    r.context,
		users:   r.users,
		code:    r.module,
		wasm:    r.wasm,
		wasi:    r.wasi,
		alone:   r.users.enter(),
	}, nil
}

// code returns the entry for the code of r.wasm: the one it had before Set
// was called with the same binary again, or a new one. It is called with r.mu
// held.
func (r *Runner) code() *compiled {
	id := sha256.Sum256(r.wasm)
	i := slices.IndexFunc(r.retired, func(old *compiled) bool { return old.id == id })
	if i < 0 {
		return &compiled{id: id}
	}
	old := r.retired[i]
	r.retired = slices.Delete(r.retired, i, i+1)
	return old
}

// release closes the code that Set replaced and no call is using. It is
// called with r.mu held.
func (r *Runner) release(ctx context.Context) error {
	var failed error
	r.retired = slices.DeleteFunc(r.retired, func(old *compiled) bool {
		if old.runs > 0 {
			return false
		}
		if old.module != nil {
			failed = errors.Join(failed, old.module.Close(ctx))
		}
		return true
	})
	return failed
}

// end is called by whoever called [Runner.begin], once it has stopped using
// what that gave it.
func (r *Runner) end(ctx context.Context, used use) error {
	// Before the lock is taken: the runtime may be this call's to close.
	err := used.users.leave(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	used.code.runs--
	if r.runtime != used.runtime {
		// Closed meanwhile, and its compiled code with it.
		return err
	}
	return errors.Join(err, r.release(ctx))
}

const (
	wasmPageSize = 65536
	// wasmMaxPages is all the memory that a guest's 32-bit addresses reach.
	wasmMaxPages = 65536
	// defaultMemory is what a guest's memory is held at unless its
	// [SystemInterface] names a limit. A guest that grows its memory without
	// end would otherwise have the host feed it until it had none left, which
	// Go can't recover from.
	defaultMemory = 64 << 20
)

// memoryLimitPages returns a limit in bytes as whole pages. A limit above what
// a guest can address is no limit on it, and wazero panics if it is given one.
func memoryLimitPages(bytes int) uint32 {
	return uint32(min(bytes/wasmPageSize, wasmMaxPages))
}

// Run runs the guest until it ends, ctx ends or the Runner is closed. A guest
// that is stopped by either of the last two makes Run return an error that
// wraps the reason: the error of ctx, or [context.Canceled] for a closed Runner.
//
// The guest is compiled by the first Run of it, unless Compile has done so, and
// the compiled code is kept for the next Run until Set or Close: Runs of one
// guest can be in progress at once, and all of them use it.
func (r *Runner) Run(ctx context.Context) (err error) {
	run, err := r.begin(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.end(ctx, run)) }()
	module, err := run.code.ready(ctx, run.runtime, run.wasm)
	if err != nil {
		return err
	}
	if run.open.Err() != nil {
		// Closed while the guest was compiled: there is nothing to stop.
		return fmt.Errorf("wasm: guest stopped: %w", context.Canceled)
	}
	wasi := run.wasi
	config := wazero.NewModuleConfig()
	if !run.alone {
		// A guest that names itself takes its name for as long as it runs,
		// and a second one of that name is refused. The only guest keeps its
		// name, as it shows in the errors of wazero; one that runs beside
		// another has none.
		config = config.WithName("")
	}
	if wasi.Stdout != nil {
		config = config.WithStdout(wasi.Stdout)
	}
	if wasi.Stderr != nil {
		config = config.WithStderr(wasi.Stderr)
	}
	if wasi.Stdin != nil {
		config = config.WithStdin(wasi.Stdin)
	}
	if wasi.Args != nil {
		config = config.WithArgs(wasi.Args...)
	} else {
		config = config.WithArgs("")
	}
	for k, v := range wasi.Env {
		config = config.WithEnv(k, v)
	}
	if wasi.Root != nil {
		config = config.WithFS(wasi.Root)
	}
	if wasi.Entropy != nil {
		config = config.WithRandSource(wasi.Entropy)
	}
	if wasi.Yield != nil {
		config = config.WithOsyield(wasi.Yield)
	}
	if wasi.Sleep != nil {
		config = config.WithNanosleep(func(ns int64) {
			wasi.Sleep(time.Duration(ns))
		})
	}
	if wasi.WallTime != nil {
		config = config.WithWalltime(wasi.WallTime, 1)
	}
	if wasi.NanoTime != nil {
		config = config.WithNanotime(wasi.NanoTime, 1)
	}
	// The guest has handles of its own, which go when it has ended, and it is
	// stopped when the runtime it runs in is closed as well as when ctx ends.
	running, stop := context.WithCancelCause(withGuest(ctx, newGuest()))
	defer stop(nil)
	defer context.AfterFunc(run.open, func() { stop(context.Canceled) })()
	guest, err := run.runtime.InstantiateModule(running, module, config)
	switch {
	case err != nil && running.Err() != nil:
		// wazero reports a guest that it stopped as an exit code of the guest's.
		return fmt.Errorf("wasm: guest stopped: %w (%w)", context.Cause(running), err)
	case err != nil && run.open.Err() != nil:
		// Closed, and the guest failed before the close had reached running.
		return fmt.Errorf("wasm: guest stopped: %w (%w)", context.Canceled, err)
	case err != nil:
		return err
	case run.open.Err() != nil:
		// A guest whose runtime was closed first ends as if it had exited.
		return fmt.Errorf("wasm: guest stopped: %w", context.Canceled)
	}
	return guest.Close(ctx)
}
