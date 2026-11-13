//go:build !wasm

package wasm

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	wasm_api "github.com/tetratelabs/wazero/api"

	"github.com/iqhive/runtime.link/api"
)

const (
	linkModule   = "github.com/iqhive/runtime.link"
	bridgeModule = "github.com/iqhive/runtime.link/ffi"
)

// How a guest declares the functions of the link and bridge modules that the
// tests below call.
var (
	oneHandle      = []wasm_api.ValueType{i64}
	opensLibrary   = signature{params: []wasm_api.ValueType{i32, i32}, results: oneHandle}
	findsSymbol    = signature{params: []wasm_api.ValueType{i64, i32, i32}, results: oneHandle}
	handleToHandle = signature{params: oneHandle, results: oneHandle}
	takesHandle    = signature{params: oneHandle}
	takesTwo       = signature{params: []wasm_api.ValueType{i64, i64}}
	twoToHandle    = signature{params: []wasm_api.ValueType{i64, i64}, results: oneHandle}
	makesString    = signature{params: []wasm_api.ValueType{i64, i32}, results: oneHandle}
)

// hostCaller calls one host function at a time the way a guest does: it links a
// forwarding guest to the function, writes text to that guest's memory at
// address 8 for the functions that read a name from there, and calls. No Run
// started these calls, so they share the handles that the runtime keeps for
// such calls, and a handle that one of them returns can be passed to the next.
type hostCaller func(module, name, text string, sig signature, args ...uint64) ([]uint64, error)

func callsOutsideRun(t *testing.T, runner *Runner) hostCaller {
	t.Helper()
	return func(module, name, text string, sig signature, args ...uint64) ([]uint64, error) {
		t.Helper()
		guest, err := runner.runtime.Instantiate(t.Context(), forwardingModule(module, name, sig))
		if err != nil {
			t.Fatalf("%s doesn't link as %v: %v", name, sig, err)
		}
		if !guest.Memory().WriteString(8, text) {
			t.Fatal("couldn't write to the guest")
		}
		return guest.ExportedFunction("call").Call(t.Context(), args...)
	}
}

// linked returns the handle that dlopen and dlsym give for a function.
func (call hostCaller) linked(t *testing.T, library, name string) uint64 {
	t.Helper()
	opened, err := call(linkModule, "dlopen", library, opensLibrary, 8, uint64(len(library)))
	if err != nil || opened[0] == 0 {
		t.Fatalf("dlopen of %s = %v, %v", library, opened, err)
	}
	function, err := call(linkModule, "dlsym", name, findsSymbol, opened[0], 8, uint64(len(name)))
	if err != nil || function[0] == 0 {
		t.Fatalf("dlsym of %s = %v, %v", name, function, err)
	}
	return function[0]
}

// arguments returns the handle that a guest sends a function's arguments to.
func (call hostCaller) arguments(t *testing.T, function uint64) uint64 {
	t.Helper()
	arguments, err := call(bridgeModule, "func_args", "", handleToHandle, function)
	if err != nil {
		t.Fatalf("func_args: %v", err)
	}
	return arguments[0]
}

func uleb(n uint64) []byte {
	var out []byte
	for {
		b := byte(n & 0x7f)
		n >>= 7
		if n == 0 {
			return append(out, b)
		}
		out = append(out, b|0x80)
	}
}

func sleb(n int64) []byte {
	var out []byte
	for {
		b := byte(n & 0x7f)
		n >>= 7
		if (n == 0 && b&0x40 == 0) || (n == -1 && b&0x40 != 0) {
			return append(out, b)
		}
		out = append(out, b|0x80)
	}
}

func sized(data []byte) []byte { return append(uleb(uint64(len(data))), data...) }

func longSection(id byte, payload []byte) []byte { return append([]byte{id}, sized(payload)...) }

func longVector(entries ...[]byte) []byte {
	return slices.Concat(append([][]byte{uleb(uint64(len(entries)))}, entries...)...)
}

type hostImport struct {
	module, name string
	sig          signature
}

const (
	localSet  = 0x21
	loop      = 0x03
	br        = 0x0C
	emptyType = 0x40
	i64Type   = 0x7E
)

// startGuest returns a guest whose _start is body, with one i64 local and one
// page of memory that holds text at address 8. Import i is function i.
func startGuest(imports []hostImport, body []byte, text string) []byte {
	var types, imported [][]byte
	for i, imp := range imports {
		types = append(types, funcSignature(imp.sig))
		imported = append(imported, slices.Concat(sized([]byte(imp.module)), sized([]byte(imp.name)), []byte{funcKind}, uleb(uint64(i))))
	}
	types = append(types, funcSignature(signature{}))
	start := uleb(uint64(len(imports)))
	code := append([]byte{1, 1, i64Type}, body...)
	return slices.Concat(
		[]byte(emptyModule),
		longSection(1, longVector(types...)),
		longSection(2, longVector(imported...)),
		longSection(3, longVector(start)),
		longSection(5, longVector([]byte{0, 1})),
		longSection(7, longVector(
			slices.Concat(sized([]byte("memory")), []byte{memKind, 0}),
			slices.Concat(sized([]byte("_start")), []byte{funcKind}, start),
		)),
		longSection(10, longVector(sized(code))),
		longSection(11, longVector(slices.Concat([]byte{0, i32Const, 8, end}, sized([]byte(text))))),
	)
}

// makesAString returns a guest that makes a string of the text in its memory
// and passes the handle it got to host.Sum, as if it had printed it.
func makesAString(text string) []byte {
	return startGuest([]hostImport{
		{bridgeModule, "string_new", makesString},
		{"host", "Sum", twoToHandle},
	}, slices.Concat([]byte{i64Const, 8, i32Const}, sleb(int64(len(text))), []byte{call, 0, i64Const, 0, call, 1, drop, end}), text)
}

// A guest names what it holds on the host by number, and a table that
// outlived it would hand those values to the next guest that guessed one.
func TestEachRunHasItsOwnHandles(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	closeRunner(t, &runner)
	var handles []int64
	var shouted []string
	runner.Add(host{
		Sum:   func(_ context.Context, a, b int64) int64 { handles = append(handles, a); return 0 },
		Shout: func(text string) string { shouted = append(shouted, text); return "" },
	})
	runner.Set(makesAString("secret"))
	if err := runner.Run(ctx); err != nil {
		t.Fatalf("the first guest: %v", err)
	}
	if len(handles) != 1 || handles[0] == 0 {
		t.Fatalf("the first guest got the handles %v", handles)
	}

	// Another module, which never made a string, uses the first one's handle.
	runner.Set(startGuest([]hostImport{
		{"host", "Shout", handleToHandle},
	}, slices.Concat([]byte{i64Const}, sleb(handles[0]), []byte{call, 0, drop, end}), ""))
	err := runner.Run(ctx)
	if len(shouted) > 0 {
		t.Fatalf("a guest used handle %d of a guest that had ended, and the host function was given %q", handles[0], shouted)
	}
	if err == nil || !strings.Contains(err.Error(), "invalid id") {
		t.Errorf("a guest that used a handle it never had ended with %v, want it trapped", err)
	}

	// What a guest left behind is gone, so the next one starts where it did.
	runner.Set(makesAString("secret"))
	if err := runner.Run(ctx); err != nil {
		t.Fatalf("the third guest: %v", err)
	}
	if len(handles) != 2 || handles[1] != handles[0] {
		t.Errorf("guests that do the same got the handles %v, want the same one: the Runner is keeping what a guest left", handles)
	}
}

func TestGuestsRunningAtOnceHoldSeparateHandles(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	closeRunner(t, &runner)
	var mu sync.Mutex
	seen := make(map[int64]int)
	runner.Add(host{Sum: func(_ context.Context, a, b int64) int64 {
		mu.Lock()
		defer mu.Unlock()
		seen[a]++
		return 0
	}})
	runner.Set(makesAString("mine"))
	if err := runner.Compile(ctx); err != nil {
		t.Fatal(err)
	}
	const guests, runs = 8, 20
	var running sync.WaitGroup
	for range guests {
		running.Go(func() {
			for range runs {
				if err := runner.Run(ctx); err != nil {
					t.Error(err)
				}
			}
		})
	}
	running.Wait()
	if len(seen) != 1 {
		t.Errorf("guests that each made one string got the handles %v, want the same one from every table", seen)
	}
	for _, count := range seen {
		if count != guests*runs {
			t.Errorf("%d guests told the host their handle, want %d", count, guests*runs)
		}
	}
}

// linksTouch returns a guest that looks host.Touch up with dlsym and then
// frees the handle, or calls it.
func linksTouch(calls bool) []byte {
	imports := []hostImport{
		{linkModule, "dlopen", opensLibrary},
		{linkModule, "dlsym", findsSymbol},
		{bridgeModule, "func_free", takesHandle},
		{bridgeModule, "func_args", handleToHandle},
		{bridgeModule, "func_call", twoToHandle},
	}
	const text = "hostTouch"
	body := []byte{i32Const, 8, i32Const, 4, call, 0, i32Const, 12, i32Const, 5, call, 1}
	if calls {
		body = append(body, localSet, 0, localGet, 0, localGet, 0, call, 3, call, 4, drop, end)
	} else {
		body = append(body, call, 2, end)
	}
	return startGuest(imports, body, text)
}

// The handle dlsym gives is the guest's own as well: freeing it takes the
// function from no other guest.
func TestGuestCannotFreeWhatAnotherGuestLinks(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	closeRunner(t, &runner)
	var touched int
	runner.Add(host{Touch: func(context.Context) { touched++ }})
	for _, calls := range []bool{true, false, true} {
		runner.Set(linksTouch(calls))
		if err := runner.Run(ctx); err != nil {
			t.Fatalf("a guest that links Touch (and calls it: %v): %v", calls, err)
		}
	}
	if touched != 2 {
		t.Errorf("Touch ran %d times, want once before and once after a guest freed its own handle to it", touched)
	}
}

// The bridge's own functions are a library that dlsym finds, so a guest can
// have func_call call func_call, with itself as the function and its own
// arguments as the arguments. That goes on for ever on the host's stack, and
// a stack overflow ends the host's whole process.
func TestFuncCallCannotGoOnCallingItself(t *testing.T) {
	var runner Runner
	closeRunner(t, &runner)
	runner.Add(host{Sum: func(_ context.Context, a, b int64) int64 { return a + b }})
	runner.Set([]byte(emptyModule))
	if err := runner.Compile(t.Context()); err != nil {
		t.Fatal(err)
	}
	call := callsOutsideRun(t, &runner)
	function := call.linked(t, bridgeModule, "func_call")
	arguments := call.arguments(t, function)
	// func_call(f, args) with f = func_call and args = these arguments.
	for _, argument := range []uint64{function, arguments} {
		if _, err := call(bridgeModule, "encode_uint64", "", takesTwo, arguments, argument); err != nil {
			t.Fatal(err)
		}
	}
	_, err := call(bridgeModule, "func_call", "", twoToHandle, function, arguments)
	if err == nil || !strings.Contains(err.Error(), "inside one another") {
		t.Fatalf("func_call calling itself returned %v, want it stopped for nesting too deep", err)
	}

	// The calls that were stopped don't count against the ones that follow.
	sum := call.linked(t, "host", "Sum")
	for range 2 {
		arguments = call.arguments(t, sum)
		for _, argument := range []uint64{40, 2} {
			if _, err := call(bridgeModule, "encode_int64", "", takesTwo, arguments, argument); err != nil {
				t.Fatal(err)
			}
		}
		results, err := call(bridgeModule, "func_call", "", twoToHandle, sum, arguments)
		if err != nil {
			t.Fatalf("a call after the stopped ones: %v", err)
		}
		if total, err := call(bridgeModule, "decode_int64", "", handleToHandle, results[0]); err != nil || total[0] != 42 {
			t.Errorf("Sum(40, 2) = %v, %v", total, err)
		}
	}
}

type references struct {
	api.Specification `api:"references"`

	Limits   func() map[string]int       `api:"limits"`
	Names    func() []string             `api:"names"`
	Counter  func() *int64               `api:"counter"`
	Events   func() chan int             `api:"events"`
	Callback func() func()               `api:"callback"`
	Anything func() any                  `api:"anything"`
	Inside   func() struct{ Tags []int } `api:"inside"`
	Listed   func() [2]*int64            `api:"listed"`
	Beside   func() (int64, []string)    `api:"beside"`

	Count func() (int64, error) `api:"count"`
}

// The package says that maps, pointers, channels, functions, interfaces and
// slice results can't cross to a guest. A guest that calls the bridge itself
// would get a handle to such a result, which is the host's own map, backing
// array or variable, and map_clear, slice_elem and encode_* write to those. So
// the host doesn't make the call.
func TestFunctionsWithResultsThatCannotCrossAreNotCalled(t *testing.T) {
	limits := map[string]int{"requests": 10}
	names := []string{"admin"}
	counter := int64(7)
	var called []string
	ran := func(name string) { called = append(called, name) }
	var runner Runner
	closeRunner(t, &runner)
	runner.Add(references{
		Limits:   func() map[string]int { ran("limits"); return limits },
		Names:    func() []string { ran("names"); return names },
		Counter:  func() *int64 { ran("counter"); return &counter },
		Events:   func() chan int { ran("events"); return make(chan int) },
		Callback: func() func() { ran("callback"); return func() {} },
		Anything: func() any { ran("anything"); return &counter },
		Inside:   func() struct{ Tags []int } { ran("inside"); return struct{ Tags []int }{} },
		Listed:   func() [2]*int64 { ran("listed"); return [2]*int64{&counter, &counter} },
		Beside:   func() (int64, []string) { ran("beside"); return 1, names },
		Count:    func() (int64, error) { ran("count"); return 3, nil },
	})
	runner.Set([]byte(emptyModule))
	if err := runner.Compile(t.Context()); err != nil {
		t.Fatal(err)
	}
	call := callsOutsideRun(t, &runner)
	for name, refused := range map[string]string{
		"limits":   "map[string]int",
		"names":    "[]string",
		"counter":  "*int64",
		"events":   "chan int",
		"callback": "func()",
		"anything": "interface {}",
		"inside":   "[]int",
		"listed":   "*int64",
		"beside":   "[]string",
	} {
		function := call.linked(t, "references", name)
		results, err := call(bridgeModule, "func_call", "", twoToHandle, function, call.arguments(t, function))
		if err == nil || !strings.Contains(err.Error(), "a "+refused+" can't cross") {
			t.Errorf("%s returned %v, %v to a guest, want it refused for its %s", name, results, err, refused)
		}
	}
	if len(called) > 0 {
		t.Errorf("the host called %v for a guest that can't have the results", called)
	}
	if len(limits) != 1 || names[0] != "admin" || counter != 7 {
		t.Errorf("the host's values are now %v, %q and %d", limits, names, counter)
	}

	// Results that cross still do, an error among them.
	function := call.linked(t, "references", "count")
	results, err := call(bridgeModule, "func_call", "", twoToHandle, function, call.arguments(t, function))
	if err != nil {
		t.Fatal(err)
	}
	if count, err := call(bridgeModule, "decode_int64", "", handleToHandle, results[0]); err != nil || count[0] != 3 {
		t.Errorf("count = %v, %v", count, err)
	}
	if !slices.Equal(called, []string{"count"}) {
		t.Errorf("the host called %v, want count alone", called)
	}
}

// A handle that the host never gave out is refused as a string argument
// (TestStringArgumentRejectsABadHandle). So is the handle of anything else
// that the guest holds: reflect has text for any value, and the function would
// be given that.
func TestStringArgumentRejectsAHandleThatIsNotAString(t *testing.T) {
	var runner Runner
	closeRunner(t, &runner)
	var got []string
	runner.Add(host{Shout: func(text string) string { got = append(got, text); return text }})
	runner.Set([]byte(emptyModule))
	if err := runner.Compile(t.Context()); err != nil {
		t.Fatal(err)
	}
	call := callsOutsideRun(t, &runner)
	function := call.linked(t, "host", "Shout")
	for what, handle := range map[string]uint64{
		"a function":             function,
		"a function's arguments": call.arguments(t, function),
	} {
		_, err := call("host", "Shout", "", handleToHandle, handle)
		if err == nil || !strings.Contains(err.Error(), "not of a string") {
			t.Errorf("the handle of %s as a string argument: %v, want it refused for its kind", what, err)
		}
	}
	if len(got) > 0 {
		t.Errorf("a function that takes a string was called with %q", got)
	}

	text, err := call(bridgeModule, "string_new", "hello", makesString, 8, 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := call("host", "Shout", "", handleToHandle, text[0]); err != nil || !slices.Equal(got, []string{"hello"}) {
		t.Errorf("the handle of a string gave the function %q, %v", got, err)
	}
	// The other functions that take the handle of a string check it too.
	if length, err := call(bridgeModule, "string_len", "", signature{params: oneHandle, results: []wasm_api.ValueType{i32}}, function); err == nil {
		t.Errorf("string_len of a function = %v", length)
	}
}
