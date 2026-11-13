//go:build !wasm

package wasm

import (
	"context"
	"math"
	"slices"
	"testing"

	"github.com/tetratelabs/wazero"
	wasm_api "github.com/tetratelabs/wazero/api"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/wasm/internal/ffi"
)

// wide has functions whose results take more than one uint64, and so can't be
// the one value a wasm function returns to a //go:wasmimport guest.
type wide struct {
	api.Specification `api:"wide"`

	Conjugate func(complex128) complex128
	Halves    func(complex64) complex64
	Both      func() [2]int64
	Point     func(int64) struct{ A, B int64 }
	Divide    func(a, b int64) (int64, int64)
	Narrow    func() (int8, bool, uint16)
	Names     func() (string, string)
	Parse     func(int64) (int64, int64, error)

	Alone   func(int64) (struct{}, int64)
	Nothing func(int64) struct{}
}

const (
	f32 = wasm_api.ValueTypeF32
	f64 = wasm_api.ValueTypeF64
)

// wideGuest links a guest to one function of a wide host, as the function is
// exported, and returns the guest with the bridge its strings are handles on.
func wideGuest(t *testing.T, impl wide, name string) (wasm_api.Module, *ffi.API, signature) {
	t.Helper()
	ctx := t.Context()
	runtime := wazero.NewRuntime(ctx)
	t.Cleanup(func() { runtime.Close(context.Background()) })
	outside := newGuest()
	child := outside.bridge
	export_api(runtime, outside, impl)
	definition := runtime.Module("wide").ExportedFunctionDefinitions()[name]
	if definition == nil {
		t.Fatalf("%s isn't exported", name)
	}
	sig := signature{params: definition.ParamTypes(), results: definition.ResultTypes()}
	guest, err := runtime.Instantiate(ctx, forwardingModule("wide", name, sig))
	if err != nil {
		t.Fatalf("%s doesn't link as %v: %v", name, sig, err)
	}
	return guest, &child, sig
}

func sameSignature(a, b signature) bool {
	return slices.Equal(a.params, b.params) && slices.Equal(a.results, b.results)
}

// slotsAt reads what the host wrote for the results: one uint64 for each slot.
func slotsAt(t *testing.T, guest wasm_api.Module, address uint32, count int) []uint64 {
	t.Helper()
	slots := make([]uint64, count)
	for i := range slots {
		slot, ok := guest.Memory().ReadUint64Le(address + uint32(i)*8)
		if !ok {
			t.Fatalf("slot %d at %d is outside the guest's memory", i, address)
		}
		slots[i] = slot
	}
	return slots
}

// The results go where the convention for exports says: to an address that the
// guest passes after its arguments, with nothing returned. Each uint64 that the
// results take is written as it would be passed as an argument.
func TestResultsWiderThanOneSlotAreWrittenToTheAddressPassedLast(t *testing.T) {
	const at = 64
	impl := wide{
		Conjugate: func(c complex128) complex128 { return complex(real(c), -imag(c)) },
		Halves:    func(c complex64) complex64 { return c / 2 },
		Both:      func() [2]int64 { return [2]int64{-1, 2} },
		Point:     func(v int64) struct{ A, B int64 } { return struct{ A, B int64 }{v, v * 2} },
		Divide:    func(a, b int64) (int64, int64) { return a / b, a % b },
		Narrow:    func() (int8, bool, uint16) { return -1, true, 65535 },
		Parse:     func(v int64) (int64, int64, error) { return v, -v, nil },
	}
	for _, test := range []struct {
		name string
		sig  signature
		args []uint64
		want []uint64
	}{
		{
			"Conjugate", signature{params: []wasm_api.ValueType{f64, f64, i64}},
			[]uint64{math.Float64bits(1.5), math.Float64bits(2.5)},
			[]uint64{math.Float64bits(1.5), math.Float64bits(-2.5)},
		},
		{
			"Halves", signature{params: []wasm_api.ValueType{f32, f32, i64}},
			[]uint64{wasm_api.EncodeF32(3), wasm_api.EncodeF32(-5)},
			[]uint64{wasm_api.EncodeF32(1.5), wasm_api.EncodeF32(-2.5)},
		},
		{"Both", signature{params: []wasm_api.ValueType{i64}}, nil, []uint64{math.MaxUint64, 2}},
		{"Point", signature{params: []wasm_api.ValueType{i64, i64}}, []uint64{21}, []uint64{21, 42}},
		{"Divide", signature{params: []wasm_api.ValueType{i64, i64, i64}}, []uint64{17, 5}, []uint64{3, 2}},
		{"Narrow", signature{params: []wasm_api.ValueType{i64}}, nil, []uint64{wasm_api.EncodeI32(-1), 1, 65535}},
		{"Parse", signature{params: []wasm_api.ValueType{i64, i64}}, []uint64{7}, []uint64{7, wasm_api.EncodeI64(-7)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			guest, _, sig := wideGuest(t, impl, test.name)
			if !sameSignature(sig, test.sig) {
				t.Fatalf("exported as %v, want %v", sig, test.sig)
			}
			// Marks the slot after the results, which the host must leave alone.
			const untouched = 0xdeadbeef
			after := uint32(at + len(test.want)*8)
			if !guest.Memory().WriteUint64Le(after, untouched) {
				t.Fatal("couldn't write to the guest")
			}
			results, err := guest.ExportedFunction("call").Call(t.Context(), append(test.args, at)...)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 0 {
				t.Errorf("returned %v beside what it wrote", results)
			}
			if got := slotsAt(t, guest, at, len(test.want)); !slices.Equal(got, test.want) {
				t.Errorf("wrote %#x, want %#x", got, test.want)
			}
			if got := slotsAt(t, guest, after, 1); got[0] != untouched {
				t.Errorf("wrote %#x past the results", got[0])
			}
		})
	}
}

func TestStringsAmongWideResultsAreHandles(t *testing.T) {
	impl := wide{Names: func() (string, string) { return "head", "tail" }}
	guest, child, _ := wideGuest(t, impl, "Names")
	if _, err := guest.ExportedFunction("call").Call(t.Context(), 8); err != nil {
		t.Fatal(err)
	}
	slots := slotsAt(t, guest, 8, 2)
	if head, tail := child.GoString(ffi.String(slots[0])), child.GoString(ffi.String(slots[1])); head != "head" || tail != "tail" {
		t.Errorf("got %q and %q", head, tail)
	}
}

// A guest that declares the function as returning its result gets no part of
// it: before, such a function linked and returned the first uint64 as if it
// were the whole value.
func TestWideResultIsNotReturnedInPart(t *testing.T) {
	ctx := t.Context()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(context.Background())
	export_api(runtime, newGuest(), wide{})
	for name, sig := range map[string]signature{
		"Conjugate": {params: []wasm_api.ValueType{f64, f64}, results: []wasm_api.ValueType{i64}},
		"Both":      {results: []wasm_api.ValueType{i64}},
		"Point":     {params: []wasm_api.ValueType{i64}, results: []wasm_api.ValueType{i64}},
		"Divide":    {params: []wasm_api.ValueType{i64, i64}, results: []wasm_api.ValueType{i64}},
	} {
		if _, err := runtime.Instantiate(ctx, forwardingModule("wide", name, sig)); err == nil {
			t.Errorf("%s links as %v, which has room for one uint64 of its results", name, sig)
		}
	}
}

// An address the results don't fit at traps the guest before the function
// runs, so that nothing happens on the host that the guest can't learn of.
func TestWideResultsAtABadAddressTrapBeforeTheCall(t *testing.T) {
	const pageSize = 65536
	for name, address := range map[string]uint64{
		"past the end":                     pageSize,
		"across the end":                   pageSize - 8,
		"address wider than memory can be": 1<<32 | 64,
	} {
		t.Run(name, func(t *testing.T) {
			var called bool
			impl := wide{Divide: func(a, b int64) (int64, int64) {
				called = true
				return a / b, a % b
			}}
			guest, _, _ := wideGuest(t, impl, "Divide")
			if _, err := guest.ExportedFunction("call").Call(t.Context(), 17, 5, address); err == nil {
				t.Error("the guest got results that don't fit in its memory")
			}
			if called {
				t.Error("the function ran with nowhere to put its results")
			}
		})
	}
	// The last address that the results do fit at.
	guest, _, _ := wideGuest(t, wide{Divide: func(a, b int64) (int64, int64) { return a / b, a % b }}, "Divide")
	if _, err := guest.ExportedFunction("call").Call(t.Context(), 17, 5, pageSize-16); err != nil {
		t.Fatal(err)
	}
	if got := slotsAt(t, guest, pageSize-16, 2); !slices.Equal(got, []uint64{3, 2}) {
		t.Errorf("wrote %v, want [3 2]", got)
	}
}

// Results that fit in one uint64 are still returned, whatever else is among them.
func TestResultsOfOneSlotAreReturned(t *testing.T) {
	impl := wide{
		Alone:   func(v int64) (struct{}, int64) { return struct{}{}, v + 1 },
		Nothing: func(int64) struct{} { return struct{}{} },
	}
	guest, _, sig := wideGuest(t, impl, "Alone")
	if want := (signature{params: []wasm_api.ValueType{i64}, results: []wasm_api.ValueType{i64}}); !sameSignature(sig, want) {
		t.Fatalf("Alone is exported as %v, want %v", sig, want)
	}
	results, err := guest.ExportedFunction("call").Call(t.Context(), 41)
	if err != nil || !slices.Equal(results, []uint64{42}) {
		t.Errorf("Alone(41) = %v, %v, want [42]", results, err)
	}
	guest, _, sig = wideGuest(t, impl, "Nothing")
	if want := (signature{params: []wasm_api.ValueType{i64}}); !sameSignature(sig, want) {
		t.Fatalf("Nothing is exported as %v, want %v", sig, want)
	}
	if results, err := guest.ExportedFunction("call").Call(t.Context(), 1); err != nil || len(results) != 0 {
		t.Errorf("Nothing(1) = %v, %v, want no results", results, err)
	}
}
