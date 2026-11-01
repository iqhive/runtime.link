//go:build !wasm

package wasm

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero"
	wasm_api "github.com/tetratelabs/wazero/api"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/wasm/internal/example"
	"github.com/iqhive/runtime.link/api/wasm/internal/ffi"
)

// host is exported to wazero exactly as [Runner.Run] exports an API, and its
// functions are called directly, which is what a //go:wasmimport guest does.
type host struct {
	api.Specification `api:"host"`

	Narrow8  func(int64) int8
	Narrow16 func(int64) int16
	Wide32   func(int64) int32

	Check     func(int64) error
	CheckNone func() error
	Parse     func(int64) (int64, error)

	Sum        func(context.Context, int64, int64) int64
	Touch      func(context.Context)
	TouchCheck func(context.Context) error

	Unset func()

	Shout func(string) string
}

// signature is how a guest declares its import of a host function. A guest
// that declares something else fails to link.
type signature struct{ params, results []wasm_api.ValueType }

const (
	i32 = wasm_api.ValueTypeI32
	i64 = wasm_api.ValueTypeI64
)

func callHost(t *testing.T, impl host, name string, sig signature, args ...uint64) ([]uint64, error) {
	t.Helper()
	return callHostWith(t, t.Context(), impl, name, sig, args...)
}

func callHostWith(t *testing.T, ctx context.Context, impl host, name string, sig signature, args ...uint64) ([]uint64, error) {
	t.Helper()
	runtime := wazero.NewRuntime(ctx)
	t.Cleanup(func() { runtime.Close(context.Background()) })
	child := ffi.New()
	export_api(runtime, &child, impl)
	guest, err := runtime.Instantiate(ctx, forwardingModule("host", name, sig))
	if err != nil {
		t.Fatalf("%s doesn't link as %v: %v", name, sig, err)
	}
	return guest.ExportedFunction("call").Call(ctx, args...)
}

func TestNarrowIntegersAreSignExtendedAcrossTheWholeSlot(t *testing.T) {
	impl := host{
		Narrow8:  func(v int64) int8 { return int8(v) },
		Narrow16: func(v int64) int16 { return int16(v) },
		Wide32:   func(v int64) int32 { return int32(v) },
	}
	narrow := signature{params: []wasm_api.ValueType{i64}, results: []wasm_api.ValueType{i32}}
	// The argument shares its stack slot with the result, so its bytes are what
	// an incomplete write leaves behind.
	const stale = 0x1122334455660000
	for _, test := range []struct {
		name string
		arg  uint64
		want int32
	}{
		{"Narrow8", stale | 0xFF, -1},
		{"Narrow8", stale | 0x80, -128},
		{"Narrow8", stale | 0x7F, 127},
		{"Narrow16", stale | 0xFFFF, -1},
		{"Narrow16", stale | 0x8000, -32768},
		{"Narrow16", stale | 0x7FFF, 32767},
		{"Wide32", stale | 0xFFFF, 0x5566FFFF},
	} {
		results, err := callHost(t, impl, test.name, narrow, test.arg)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if got := wasm_api.DecodeI32(results[0]); got != test.want {
			t.Errorf("%s(%#x) = %d, want %d", test.name, test.arg, got, test.want)
		}
	}
}

func TestErrorResultIsNotAWasmResult(t *testing.T) {
	impl := host{
		Check:      func(int64) error { return nil },
		CheckNone:  func() error { return nil },
		TouchCheck: func(context.Context) error { return nil },
	}
	takesOne := signature{params: []wasm_api.ValueType{i64}}
	for _, test := range []struct {
		name string
		sig  signature
		args []uint64
	}{
		{"Check", takesOne, []uint64{1}},
		{"CheckNone", signature{}, nil},
		{"TouchCheck", signature{}, nil},
	} {
		results, err := callHost(t, impl, test.name, test.sig, test.args...)
		if err != nil {
			t.Errorf("%s: a nil error should reach the guest as a normal return: %v", test.name, err)
		}
		if len(results) != 0 {
			t.Errorf("%s: results = %v, want none", test.name, results)
		}
	}
}

func TestValueBesideErrorIsReturned(t *testing.T) {
	impl := host{Parse: func(v int64) (int64, error) { return v * 2, nil }}
	sig := signature{params: []wasm_api.ValueType{i64}, results: []wasm_api.ValueType{i64}}
	results, err := callHost(t, impl, "Parse", sig, 21)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(results, []uint64{42}) {
		t.Fatalf("results = %v, want [42]", results)
	}
}

func TestErrorTrapsTheGuest(t *testing.T) {
	failure := errors.New("host said no")
	impl := host{
		Check:      func(int64) error { return failure },
		CheckNone:  func() error { return failure },
		TouchCheck: func(context.Context) error { return failure },
		Parse:      func(int64) (int64, error) { return 0, failure },
	}
	takesOne := signature{params: []wasm_api.ValueType{i64}}
	for _, test := range []struct {
		name string
		sig  signature
		args []uint64
	}{
		{"Check", takesOne, []uint64{1}},
		{"CheckNone", signature{}, nil},
		{"TouchCheck", signature{}, nil},
		{"Parse", signature{params: takesOne.params, results: takesOne.params}, []uint64{1}},
	} {
		if _, err := callHost(t, impl, test.name, test.sig, test.args...); !errors.Is(err, failure) {
			t.Errorf("%s: err = %v, want %v", test.name, err, failure)
		}
	}
}

func TestUnsetFunctionTrapsTheGuest(t *testing.T) {
	if _, err := callHost(t, host{}, "Unset", signature{}); !errors.Is(err, api.ErrNotImplemented) {
		t.Fatalf("err = %v, want %v", err, api.ErrNotImplemented)
	}
}

type contextKey struct{}

func TestContextIsNotAWasmParameter(t *testing.T) {
	ctx := context.WithValue(t.Context(), contextKey{}, "guest")
	var seen any
	impl := host{
		Sum: func(ctx context.Context, a, b int64) int64 {
			seen = ctx.Value(contextKey{})
			return a + b
		},
		Touch: func(ctx context.Context) { seen = ctx.Value(contextKey{}) },
	}
	sig := signature{params: []wasm_api.ValueType{i64, i64}, results: []wasm_api.ValueType{i64}}
	results, err := callHostWith(t, ctx, impl, "Sum", sig, 40, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(results, []uint64{42}) {
		t.Errorf("results = %v, want [42]", results)
	}
	if seen != "guest" {
		t.Errorf("Sum saw context value %v", seen)
	}

	seen = nil
	if _, err := callHostWith(t, ctx, impl, "Touch", signature{}); err != nil {
		t.Fatal(err)
	}
	if seen != "guest" {
		t.Errorf("Touch saw context value %v", seen)
	}
}

// emptyModule is the smallest module wazero accepts.
const emptyModule = "\x00asm\x01\x00\x00\x00"

const (
	clockRealtime  = 0
	clockMonotonic = 1
)

func TestStringArgumentIsAHandle(t *testing.T) {
	ctx := t.Context()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(context.Background())
	child := ffi.New()
	export_api(runtime, &child, host{Shout: strings.ToUpper})
	sig := signature{params: []wasm_api.ValueType{i64}, results: []wasm_api.ValueType{i64}}
	guest, err := runtime.Instantiate(ctx, forwardingModule("host", "Shout", sig))
	if err != nil {
		t.Fatal(err)
	}
	results, err := guest.ExportedFunction("call").Call(ctx, uint64(child.NewString("hello")))
	if err != nil {
		t.Fatal(err)
	}
	if got := child.GoString(ffi.String(results[0])); got != "HELLO" {
		t.Fatalf("got %q", got)
	}
}

func TestStringArgumentRejectsABadHandle(t *testing.T) {
	impl := host{Shout: strings.ToUpper}
	sig := signature{params: []wasm_api.ValueType{i64}, results: []wasm_api.ValueType{i64}}
	if _, err := callHost(t, impl, "Shout", sig, 12345); err == nil {
		t.Fatal("a handle the host never gave out reached the function")
	}
}

// stringNew links a guest to string_new, which reads the guest's memory.
func stringNew(t *testing.T) (*ffi.API, wasm_api.Module) {
	t.Helper()
	ctx := t.Context()
	runtime := wazero.NewRuntime(ctx)
	t.Cleanup(func() { runtime.Close(context.Background()) })
	child := ffi.New()
	export_api(runtime, &child, child)
	sig := signature{params: []wasm_api.ValueType{i64, i32}, results: []wasm_api.ValueType{i64}}
	guest, err := runtime.Instantiate(ctx, forwardingModule(api.StructureOf(child).Name, "string_new", sig))
	if err != nil {
		t.Fatal(err)
	}
	return &child, guest
}

func TestStringNewReadsGuestMemory(t *testing.T) {
	child, guest := stringNew(t)
	if !guest.Memory().WriteString(8, "hello") {
		t.Fatal("couldn't write to the guest")
	}
	results, err := guest.ExportedFunction("call").Call(t.Context(), 8, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got := child.GoString(ffi.String(results[0])); got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestStringNewStopsAtTheEndOfGuestMemory(t *testing.T) {
	_, guest := stringNew(t)
	const pageSize = 65536
	for _, test := range []struct {
		name         string
		offset, size uint64
	}{
		{"past the end", pageSize + 1, 1},
		{"across the end", pageSize - 1, 2},
		{"offset wider than memory can be", 1<<32 | 8, 5},
	} {
		if _, err := guest.ExportedFunction("call").Call(t.Context(), test.offset, test.size); err == nil {
			t.Errorf("%s: the guest got a string out of it", test.name)
		}
	}
}

func TestNamesAreAttachedWhereTheyLineUp(t *testing.T) {
	ctx := t.Context()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(context.Background())
	child := ffi.New()
	export_api(runtime, &child, example.API{})
	definitions := runtime.Module("example").ExportedFunctionDefinitions()
	for _, test := range []struct {
		name            string
		params, results []string
	}{
		{name: "Add", params: []string{"a", "b"}},
		{name: "Echo", params: []string{"text"}, results: []string{"echoed"}},
		{name: "Sum", params: []string{"values"}},
		{name: "Split", params: []string{"text"}}, // two results spill into one slot
		{name: "Conjugate"},                       // no name for a complex128's second slot
		{name: "Move"},                            // nor for those of the struct and array
		{name: "Scalars"},                         // nor for one of six parameters
		{name: "Spread"},                          // two names, two slots, but not one each
		{name: "Partly"},                          // one name short of a full list
		{name: "HostArch"},                        // the source names nothing
	} {
		definition := definitions[test.name]
		if definition == nil {
			t.Fatalf("%s isn't exported", test.name)
		}
		if got := definition.ParamNames(); !slices.Equal(got, test.params) {
			t.Errorf("%s: param names = %v, want %v", test.name, got, test.params)
		}
		if got := definition.ResultNames(); !slices.Equal(got, test.results) {
			t.Errorf("%s: result names = %v, want %v", test.name, got, test.results)
		}
	}
}

const (
	funcType = 0x60
	funcKind = 0x00
	memKind  = 0x02
)

func section(id byte, payload []byte) []byte {
	return append([]byte{id, byte(len(payload))}, payload...)
}

func wasmName(s string) []byte { return append([]byte{byte(len(s))}, s...) }

func vector(entries ...[]byte) []byte {
	return slices.Concat(append([][]byte{{byte(len(entries))}}, entries...)...)
}

func funcSignature(sig signature) []byte {
	return slices.Concat(
		[]byte{funcType, byte(len(sig.params))}, sig.params,
		[]byte{byte(len(sig.results))}, sig.results,
	)
}

const (
	localGet = 0x20
	i32Const = 0x41
	i64Const = 0x42
	call     = 0x10
	drop     = 0x1A
	end      = 0x0B
	noLocals = 0
)

// forwardingModule returns a guest whose "call" export passes its arguments to
// a function of a host module, which is what a //go:wasmimport does. wazero
// won't let a test call the host function directly.
func forwardingModule(module, name string, sig signature) []byte {
	body := []byte{noLocals}
	for i := range sig.params {
		body = append(body, localGet, byte(i))
	}
	body = append(body, call, 0, end)
	return slices.Concat(
		[]byte(emptyModule),
		section(1, vector(funcSignature(sig))),
		section(2, vector(slices.Concat(wasmName(module), wasmName(name), []byte{funcKind, 0}))),
		section(3, vector([]byte{0})),
		section(5, vector([]byte{0, 1})), // one page of memory, no maximum
		section(7, vector(
			slices.Concat(wasmName("call"), []byte{funcKind, 1}),
			slices.Concat(wasmName("memory"), []byte{memKind, 0}),
		)),
		section(10, vector(append([]byte{byte(len(body))}, body...))),
	)
}

// namedModule returns an empty module with a name section, which makes wazero
// treat its instances as one named module.
func namedModule(name string) []byte {
	payload := slices.Concat(wasmName("name"), section(0, wasmName(name)))
	return slices.Concat([]byte(emptyModule), section(0, payload))
}

// clockModule returns a module whose _start reads the given WASI clock, the
// simplest guest that depends on a [SystemInterface] clock.
func clockModule(clock byte) []byte {
	clockTimeGet := signature{params: []wasm_api.ValueType{i32, i64, i32}, results: []wasm_api.ValueType{i32}}
	body := []byte{noLocals, i32Const, clock, i64Const, 0, i32Const, 0, call, 0, drop, end}
	return slices.Concat(
		[]byte(emptyModule),
		section(1, vector(funcSignature(clockTimeGet), funcSignature(signature{}))),
		section(2, vector(slices.Concat(wasmName("wasi_snapshot_preview1"), wasmName("clock_time_get"), []byte{funcKind, 0}))),
		section(3, vector([]byte{1})),    // _start has the second type
		section(5, vector([]byte{0, 1})), // one page of memory, no maximum
		section(7, vector(
			slices.Concat(wasmName("memory"), []byte{memKind, 0}),
			slices.Concat(wasmName("_start"), []byte{funcKind, 1}),
		)),
		section(10, vector(append([]byte{byte(len(body))}, body...))),
	)
}
