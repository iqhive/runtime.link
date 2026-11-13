package ffi_test

import (
	"context"
	"errors"
	"math"
	"reflect"
	"slices"
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

// unset returns the names of the functions that a struct of functions, or of
// structs of them, leaves nil.
func unset(value reflect.Value, prefix string) []string {
	var names []string
	for i := range value.NumField() {
		field, name := value.Field(i), prefix+value.Type().Field(i).Name
		switch field.Kind() {
		case reflect.Func:
			if field.IsNil() {
				names = append(names, name)
			}
		case reflect.Struct:
			names = append(names, unset(field, name+".")...)
		}
	}
	return names
}

// The bridge reflects on types and makes values of them, as it always has.
func TestBridgeReflectsOnTypesAndMakesValuesOfThem(t *testing.T) {
	bridge := ffi.New()
	for name, part := range map[string]any{
		"Type.":      bridge.Type,
		"Field.":     bridge.Field,
		"Slice.":     bridge.Slice,
		"Channel.":   bridge.Channel,
		"Pointer.":   bridge.Pointer,
		"Map.":       bridge.Map,
		"Structure.": bridge.Structure,
		"String.":    bridge.String,
		"Function.":  bridge.Function,
	} {
		left := slices.DeleteFunc(unset(reflect.ValueOf(part), name), func(name string) bool {
			return name == "Map.Get" // never set
		})
		if len(left) > 0 {
			t.Errorf("%v are unset", left)
		}
	}
	f := bridge.NewFunction(func([]int64, chan int64, map[string]int64) {})
	fn := bridge.Structure.Type(ffi.Structure(f))
	if kind := bridge.Type.Kind(fn); kind != reflect.Func {
		t.Fatalf("the type of a function is a %s", kind)
	}
	slice := bridge.Type.Arg(fn, 0)
	if made := bridge.Slice.Make(slice, 4, 8); bridge.Slice.Len(made) != 4 || bridge.Slice.Cap(made) != 8 {
		t.Errorf("Slice.Make(4, 8) made one of %d, %d", bridge.Slice.Len(made), bridge.Slice.Cap(made))
	}
	if made := bridge.Channel.Make(bridge.Type.Arg(fn, 1), 8); bridge.Channel.Cap(made) != 8 {
		t.Errorf("Channel.Make(8) made one of %d", bridge.Channel.Cap(made))
	}
	if made := bridge.Map.Make(bridge.Type.Arg(fn, 2)); bridge.Map.Len(made) != 0 {
		t.Errorf("Map.Make made one of %d", bridge.Map.Len(made))
	}
	if made := bridge.Pointer.New(slice); !bridge.Pointer.Nil(made) {
		t.Error("Pointer.New of a slice type isn't a nil slice")
	}
}

// The guest names both the type and how many of it, so one call could have
// the host allocate more than it has.
func TestMadeValuesAreBounded(t *testing.T) {
	bridge := ffi.New()
	f := bridge.NewFunction(func([]int64, chan int64, [4 << 20]int64, []struct{}) {})
	fn := bridge.Structure.Type(ffi.Structure(f))
	mustPanic(t, "Slice.Make of 32MiB", func() { bridge.Slice.Make(bridge.Type.Arg(fn, 0), 0, 4<<20) })
	mustPanic(t, "Slice.Make of 32MiB long", func() { bridge.Slice.Make(bridge.Type.Arg(fn, 0), 4<<20, 0) })
	mustPanic(t, "Channel.Make of 32MiB", func() { bridge.Channel.Make(bridge.Type.Arg(fn, 1), 4<<20) })
	mustPanic(t, "Pointer.New of 32MiB", func() { bridge.Pointer.New(bridge.Type.Arg(fn, 2)) })
	// Values of no size take nothing however many there are.
	if made := bridge.Slice.Make(bridge.Type.Arg(fn, 3), math.MaxUint32, math.MaxUint32); bridge.Slice.Len(made) != math.MaxUint32 {
		t.Errorf("Slice.Make of empty structs made %d", bridge.Slice.Len(made))
	}
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

// A handle is a number to a guest, so it can pass the handle of one thing where
// another is expected. reflect answers some questions about a value of any
// kind, which must not turn into an answer for the guest.
func TestHandleOfAnotherKindIsRefused(t *testing.T) {
	bridge := ffi.New()
	text := bridge.NewString("text")
	function := bridge.NewFunction(func(string) {})
	arguments := bridge.Function.Args(function)

	if got := bridge.GoString(text); got != "text" {
		t.Fatalf("GoString = %q", got)
	}
	for name, use := range map[string]func(){
		"GoString of a function":      func() { bridge.GoString(ffi.String(function)) },
		"GoString of arguments":       func() { bridge.GoString(ffi.String(arguments)) },
		"String.Len of a function":    func() { bridge.String.Len(ffi.String(function)) },
		"String.Data of a function":   func() { bridge.String.Data(ffi.String(function)) },
		"Encode.String of a function": func() { bridge.Structure.Encode.String(arguments, ffi.String(function)) },
		"Function.Args of a string":   func() { bridge.Function.Args(ffi.Function(text)) },
		"Function.Call of a string":   func() { bridge.Function.Call(t.Context(), ffi.Function(text), arguments) },
		"Function.Nil of a string":    func() { bridge.Function.Nil(ffi.Function(text)) },
		"Slice.Len of a string":       func() { bridge.Slice.Len(ffi.Slice(text)) },
		"Slice.From of a string":      func() { bridge.Slice.From(ffi.Slice(text), 0, 1) },
		"Slice.Elem of a string":      func() { bridge.Slice.Elem(ffi.Slice(text), 0) },
		"Map.Len of a string":         func() { bridge.Map.Len(ffi.Map(text)) },
		"Channel.Len of a string":     func() { bridge.Channel.Len(ffi.Channel(text)) },
	} {
		mustPanic(t, name, use)
	}
	// The refused string didn't count as the function's argument.
	bridge.Structure.Encode.String(arguments, text)
}

// Only func_call nests a call inside another, when it is the function called,
// and a guest picks both the function and the arguments: the function can be
// func_call and the arguments their own handle, for ever.
func TestFuncCallNestingIsBounded(t *testing.T) {
	bridge := ffi.New()
	ctx := t.Context()
	callsItself := bridge.NewFunction(bridge.Function.Call)
	endless := bridge.Function.Args(callsItself)
	bridge.Structure.Encode.Uint64(endless, uint64(callsItself))
	bridge.Structure.Encode.Uint64(endless, uint64(endless))
	mustPanic(t, "func_call of func_call of itself", func() { bridge.Function.Call(ctx, callsItself, endless) })

	// A call of func_call that ends is still a call, also after the ones above.
	var ran int
	counts := bridge.NewFunction(func() { ran++ })
	for range 2 {
		once := bridge.Function.Args(callsItself)
		bridge.Structure.Encode.Uint64(once, uint64(counts))
		bridge.Structure.Encode.Uint64(once, 0)
		bridge.Function.Call(ctx, callsItself, once)
	}
	if ran != 2 {
		t.Fatalf("a function called through func_call ran %d times, want 2", ran)
	}
}

// What the guest side refuses to read (TestReferenceResultsAreRefusedByName),
// the host doesn't hand out: a guest that calls the bridge itself would get a
// handle to the host's own map, backing array or variable.
func TestFunctionWithResultsThatCannotCrossIsNotCalled(t *testing.T) {
	number := 1
	for name, fn := range map[string]any{
		"map[string]int": func() map[string]int { number++; return nil },
		"[]string":       func() []string { number++; return nil },
		"*int":           func() *int { number++; return &number },
		"chan int":       func() chan int { number++; return nil },
		"func()":         func() func() { number++; return nil },
		"interface {}":   func() any { number++; return &number },
		"[]int": func() (int, struct{ Inner [1]struct{ Tags []int } }, error) {
			number++
			return 0, struct{ Inner [1]struct{ Tags []int } }{}, nil
		},
	} {
		bridge := ffi.New()
		function := bridge.NewFunction(fn)
		arguments := bridge.Function.Args(function)
		refused := func(what string, call func()) {
			t.Helper()
			defer func() {
				err, _ := recover().(error)
				if err == nil || !strings.Contains(err.Error(), "a "+name+" can't cross") {
					t.Errorf("%s of a function that returns a %s: %v, want it refused by name", what, name, err)
				}
			}()
			call()
		}
		refused("the host's call", func() { bridge.Function.Call(t.Context(), function, arguments) })
		// The guest's side says so before it sends anything.
		before := bridge.LiveHandles()
		refused("the guest's call", func() {
			bridge.Invoke(t.Context(), api.Function{Type: reflect.TypeOf(fn)}, function, nil)
		})
		if after := bridge.LiveHandles(); after != before {
			t.Errorf("the refused call of a function that returns a %s left %d handles behind", name, after-before)
		}
	}
	if number != 1 {
		t.Fatalf("a function ran for results that can't cross")
	}

	// Empty arrays and structs hold nothing that could not cross.
	var got struct {
		None  [0]*int
		Count int8
	}
	type result = struct {
		None  [0]*int
		Count int8
	}
	call(t, t.Context(), func() result { return result{Count: 3} }, nil, &got)
	if got.Count != 3 {
		t.Fatalf("got %+v", got)
	}
}
