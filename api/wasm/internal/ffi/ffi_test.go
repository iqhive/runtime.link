package ffi_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/wasm/internal/ffi"
)

func mustPanic(t *testing.T, what string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s: no panic", what)
		}
	}()
	fn()
}

func TestFreedHandleIsGone(t *testing.T) {
	bridge := ffi.New()
	handle := bridge.NewString("hello")
	if got := bridge.String.Len(handle); got != 5 {
		t.Fatalf("Len = %d", got)
	}
	bridge.String.Free(handle)
	mustPanic(t, "freed string", func() { bridge.String.Len(handle) })
}

func TestHandlesBelongToOneBridge(t *testing.T) {
	one, other := ffi.New(), ffi.New()
	secret := one.NewString("secret")
	mustPanic(t, "another bridge's string", func() { other.String.Len(secret) })
}

// call makes a call the way a guest does, into results, and checks that it left
// nothing but the function's own handle behind.
func call(t *testing.T, ctx context.Context, fn any, args []any, results ...any) error {
	t.Helper()
	bridge := ffi.New()
	values := make([]reflect.Value, len(args))
	for i, arg := range args {
		values[i] = reflect.ValueOf(arg)
	}
	function := api.Function{Type: reflect.TypeOf(fn)}
	outs := bridge.Invoke(ctx, function, bridge.NewFunction(fn), values)
	if live := bridge.LiveHandles(); live != 1 {
		t.Errorf("the call left %d handles behind", live-1)
	}
	for i, result := range results {
		reflect.ValueOf(result).Elem().Set(outs[i])
	}
	if function.NumOut() == function.Type.NumOut() {
		return nil
	}
	err, _ := outs[len(outs)-1].Interface().(error)
	return err
}

type (
	celsius float32
	label   string
)

type inner struct {
	Pair   [2]int16
	hidden uint8
}

type outer struct {
	Name  label
	Inner inner
	Empty struct{}
	Tags  []string
	Done  bool
}

func TestScalarArgumentsAndResults(t *testing.T) {
	var (
		a      int8
		b      uint16
		c      bool
		d      float32
		e      complex64
		f      uintptr
		g      celsius
		h      int
		i      complex128
		maxInt = int64(1<<63 - 1)
		j      int64
	)
	call(t, t.Context(),
		func(a int8, b uint16, c bool, d float32, e complex64, f uintptr, g celsius, h int, i complex128, j int64) (int8, uint16, bool, float32, complex64, uintptr, celsius, int, complex128, int64) {
			return a, b, c, d, e, f, g, h, i, j
		},
		[]any{int8(-128), uint16(65535), true, float32(1.5), complex64(2 + 3i), uintptr(7), celsius(36.6), -1, 4 - 5i, maxInt},
		&a, &b, &c, &d, &e, &f, &g, &h, &i, &j,
	)
	if a != -128 || b != 65535 || !c || d != 1.5 || e != 2+3i || f != 7 || g != 36.6 || h != -1 || i != 4-5i || j != maxInt {
		t.Fatal(a, b, c, d, e, f, g, h, i, j)
	}
}

func TestStringsKeepEveryByte(t *testing.T) {
	for _, text := range []string{"", "x", "a\x00b\xff|&\\", strings.Repeat("long", 100)} {
		var got label
		call(t, t.Context(), func(s label) label { return s }, []any{label(text)}, &got)
		if string(got) != text {
			t.Fatalf("got %q, want %q", got, text)
		}
	}
}

func TestStructsAndArraysCrossWhole(t *testing.T) {
	in := outer{Name: "n", Inner: inner{Pair: [2]int16{-1, 2}, hidden: 9}, Tags: []string{"t"}, Done: true}
	var seen outer
	var out inner
	call(t, t.Context(),
		func(o outer, arr [3]inner) inner {
			seen = o
			return arr[2]
		},
		[]any{in, [3]inner{2: {hidden: 3}}},
		&out,
	)
	if !reflect.DeepEqual(seen, in) {
		t.Fatalf("host saw %+v, want %+v", seen, in)
	}
	if out.hidden != 3 {
		t.Fatalf("result = %+v", out)
	}
}

func TestSlicesAreCopiesThatKeepNil(t *testing.T) {
	var seen [][]int8
	fn := func(rows ...[]int8) { seen = rows }
	sent := [][]int8{{1, -2}, nil, {}, {3}}
	call(t, t.Context(), fn, []any{sent})
	if !reflect.DeepEqual(seen, sent) {
		t.Fatalf("host saw %#v, want %#v", seen, sent)
	}
	if seen[1] != nil || seen[2] == nil {
		t.Fatalf("nil and empty got mixed up: %#v", seen)
	}
	seen[0][0] = 100
	if sent[0][0] != 1 {
		t.Fatal("the host wrote through to the guest's slice")
	}
}

func TestSliceOfNothingHasItsLength(t *testing.T) {
	var got int
	call(t, t.Context(), func(s []struct{}) int { return len(s) }, []any{make([]struct{}, 3)}, &got)
	if got != 3 {
		t.Fatalf("got %d", got)
	}
}

func TestClaimedSliceLengthAllocatesNothing(t *testing.T) {
	bridge := ffi.New()
	function := bridge.NewFunction(func([]int64) {})
	pargs := bridge.Function.Args(function)
	bridge.Structure.Encode.Int64(pargs, 1<<62)
	bridge.Structure.Encode.Int64(pargs, 1)
}

func TestContextComesFromTheCall(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(t.Context(), key{}, "call")
	var got string
	call(t, ctx,
		func(ctx context.Context, suffix string) string { return ctx.Value(key{}).(string) + suffix },
		[]any{context.Background(), "!"},
		&got,
	)
	if got != "call!" {
		t.Fatalf("got %q", got)
	}
}

func TestErrorsCross(t *testing.T) {
	for _, test := range []struct {
		err     error
		message string
	}{
		{nil, ""},
		{errors.New("no"), "no"},
		{errors.New(""), ""},
	} {
		var value int
		err := call(t, t.Context(), func() (int, error) { return 1, test.err }, nil, &value)
		if value != 1 || (err == nil) != (test.err == nil) {
			t.Errorf("%v: got %d, %v", test.err, value, err)
		}
		if err != nil && err.Error() != test.message {
			t.Errorf("message = %q, want %q", err, test.message)
		}
	}
}

func TestBareErrorCrosses(t *testing.T) {
	fails := func(text string) error { return errors.New(text) }
	if err := call(t, t.Context(), fails, []any{"no"}); err == nil || err.Error() != "no" {
		t.Fatalf("err = %v", err)
	}
	succeeds := func(string) error { return nil }
	if err := call(t, t.Context(), succeeds, []any{"yes"}); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestSeveralResultsCross(t *testing.T) {
	var a int
	var b string
	var c [2]uint8
	call(t, t.Context(), func() (int, string, [2]uint8) { return 1, "two", [2]uint8{3, 4} }, nil, &a, &b, &c)
	if a != 1 || b != "two" || c != [2]uint8{3, 4} {
		t.Fatal(a, b, c)
	}
}

func TestFunctionWithNothingToCross(t *testing.T) {
	var ran bool
	call(t, t.Context(), func() { ran = true }, nil)
	if !ran {
		t.Fatal("the function didn't run")
	}
}

func TestRefusedArgumentLeavesNoHandles(t *testing.T) {
	bridge := ffi.New()
	fn := func(string, *int) {}
	function := bridge.NewFunction(fn)
	mustPanic(t, "pointer argument", func() {
		bridge.Invoke(t.Context(), api.Function{Type: reflect.TypeOf(fn)}, function, []reflect.Value{
			reflect.ValueOf("text"), reflect.ValueOf(new(int)),
		})
	})
	if live := bridge.LiveHandles(); live != 1 {
		t.Fatalf("the refused call left %d handles behind", live-1)
	}
}

func TestReferencesAreRefusedBeforeTheyCross(t *testing.T) {
	bridge := ffi.New()
	pargs := bridge.Function.Args(bridge.NewFunction(func(any) {}))
	number := 1
	for _, value := range []any{
		map[string]int{},
		&number,
		make(chan int),
		func() {},
		[]*int{&number},
		struct{ P *int }{&number},
	} {
		mustPanic(t, reflect.TypeOf(value).String(), func() { bridge.Encode(pargs, reflect.ValueOf(value)) })
	}
}

func TestReferenceResultsAreRefusedByName(t *testing.T) {
	bridge := ffi.New()
	var got []string
	defer func() {
		if message := recover(); message == nil || !strings.Contains(message.(error).Error(), "[]string") {
			t.Fatalf("panic = %v", message)
		}
	}()
	bridge.Decode(&got, 0)
}

func TestTooManyArgumentsArePanics(t *testing.T) {
	bridge := ffi.New()
	pargs := bridge.Function.Args(bridge.NewFunction(func(int) {}))
	bridge.Structure.Encode.Int64(pargs, 1)
	mustPanic(t, "extra argument", func() { bridge.Structure.Encode.Int64(pargs, 2) })
}

func TestStringNewFailsOnAShortRead(t *testing.T) {
	bridge := ffi.New()
	mustPanic(t, "short read", func() { bridge.String.New(strings.NewReader("ab"), 3) })
}
