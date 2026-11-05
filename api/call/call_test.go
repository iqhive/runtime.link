//go:build cgo

package call

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/call/internal/dll"
)

var helpers struct {
	once sync.Once
	dir  string
	libs []string
	err  error
}

func TestMain(m *testing.M) {
	code := m.Run()
	if helpers.dir != "" {
		os.RemoveAll(helpers.dir)
	}
	os.Exit(code)
}

// compilers returns the shared library built by each installed C compiler.
func compilers(t *testing.T) []string {
	t.Helper()
	helpers.once.Do(func() {
		helpers.dir, helpers.err = os.MkdirTemp("", "call-helpers")
		if helpers.err != nil {
			return
		}
		for _, cc := range []string{"gcc", "clang"} {
			if _, err := exec.LookPath(cc); err != nil {
				continue
			}
			lib := filepath.Join(helpers.dir, cc+".so")
			out, err := exec.Command(cc, "-O2", "-shared", "-fPIC", "-o", lib, "testdata/helpers.c").CombinedOutput()
			if err != nil {
				helpers.err = fmt.Errorf("%s: %w: %s", cc, err, out)
				return
			}
			helpers.libs = append(helpers.libs, lib)
		}
	})
	if helpers.err != nil {
		t.Fatal(helpers.err)
	}
	if len(helpers.libs) == 0 {
		t.Skip("requires a C compiler")
	}
	return helpers.libs
}

func open(t *testing.T, lib string) dll.SymbolTable {
	t.Helper()
	table, err := dll.Open(lib)
	if err != nil {
		t.Skip("requires dynamic loading: ", err)
	}
	return table
}

func symbol(t *testing.T, lib, name string) unsafe.Pointer {
	t.Helper()
	ptr, err := dll.Sym(open(t, lib), name)
	if err != nil {
		t.Fatal(err)
	}
	return ptr
}

// helper returns a function from the first compiler's library.
func helper(t *testing.T, name string) unsafe.Pointer {
	t.Helper()
	return symbol(t, compilers(t)[0], name)
}

func libc(t *testing.T, name string) unsafe.Pointer {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("requires glibc")
	}
	return symbol(t, "libc.so.6", name)
}

func makeFn[T any](t *testing.T, jump unsafe.Pointer, tag string) T {
	t.Helper()
	fn, err := Make[T](jump, tag)
	if err != nil {
		t.Fatal(err)
	}
	return fn
}

func recovered(fn func()) (r any) {
	defer func() { r = recover() }()
	fn()
	return nil
}

func invoke(t *testing.T, ptr unsafe.Pointer, in []reflect.Value, out ...reflect.Type) []reflect.Value {
	t.Helper()
	vals, _, err := platform{}.Call(ptr, in, out...)
	if err != nil {
		t.Fatal(err)
	}
	return vals
}

type namedSymbols struct {
	api.Specification
	LastMissing  func(int32) int32 `call:"abs,does_not_exist func(int)int"`
	BothResolved func(int32) int32 `call:"preferred,fallback func(int)int"`
}

func TestFirstResolvableNameIsNotForgotten(t *testing.T) {
	abs := libc(t, "abs")
	var lib namedSymbols
	err := API.Link(api.StructureOf(&lib), "", Options{LookupSymbol: func(name string) (unsafe.Pointer, error) {
		if name == "abs" {
			return abs, nil
		}
		return nil, errors.New("no symbol " + name)
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := lib.LastMissing(-5); got != 5 {
		t.Errorf("abs(-5) = %d", got)
	}
}

// The last name that resolves is the one that 840f71d bound, so it still is.
func TestLastResolvableNameWins(t *testing.T) {
	toupper, tolower := libc(t, "toupper"), libc(t, "tolower")
	var looked []string
	var lib namedSymbols
	err := API.Link(api.StructureOf(&lib), "", Options{LookupSymbol: func(name string) (unsafe.Pointer, error) {
		looked = append(looked, name)
		switch name {
		case "abs":
			return libc(t, "abs"), nil
		case "preferred":
			return toupper, nil
		case "fallback":
			return tolower, nil
		}
		return nil, errors.New("no symbol " + name)
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := lib.BothResolved('A'); got != 'a' {
		t.Errorf("the first name won: %q", rune(got))
	}
	if want := []string{"abs", "does_not_exist", "preferred", "fallback"}; !reflect.DeepEqual(looked, want) {
		t.Errorf("looked up %v, want %v", looked, want)
	}
}

type twoNames struct {
	api.Specification
	Upper func(int32) int32 `call:"first,second func(int)int"`
	Abs   func(int32) error `call:"first,second func(int)int"`
}

func TestLookupAddressIsUsedDespiteItsError(t *testing.T) {
	toupper := libc(t, "toupper")
	var lib twoNames
	err := API.Link(api.StructureOf(&lib), "", Options{LookupSymbol: func(name string) (unsafe.Pointer, error) {
		if name == "second" {
			return toupper, errors.New("a warning")
		}
		return nil, errors.New("no symbol " + name)
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := lib.Upper('a'); got != 'A' {
		t.Errorf("toupper('a') = %q", rune(got))
	}
}

func TestUnresolvedSymbolReportsTheLastLookupError(t *testing.T) {
	var lib twoNames
	err := API.Link(api.StructureOf(&lib), "", Options{LookupSymbol: func(name string) (unsafe.Pointer, error) {
		return nil, errors.New("no symbol " + name)
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := lib.Abs(1); err == nil || err.Error() != "no symbol second" {
		t.Errorf("error = %v, want the error of the last lookup", err)
	}
	if r := recovered(func() { lib.Upper(1) }); fmt.Sprint(r) != "no symbol second" {
		t.Errorf("panic = %v, want the error of the last lookup", r)
	}
}

type cosLibrary struct {
	api.Specification
	Cos func(float64) float64 `call:"cos func(double)double"`
}

func TestLaterLibraryDoesNotForgetSymbol(t *testing.T) {
	var lib cosLibrary
	if err := API.Link(api.StructureOf(&lib), "libm.so.6 "+compilers(t)[0], Options{}); err != nil {
		t.Fatal(err)
	}
	if got := lib.Cos(0); got != 1 {
		t.Errorf("cos(0) = %v", got)
	}
}

type compilerLibrary struct {
	api.Specification
	Clang func() int32 `call:"compiled_by_clang func()int"`
}

// builtByClang asks the library itself, because a compiler's name does not say
// which compiler it is.
func builtByClang(t *testing.T, lib string) int64 {
	t.Helper()
	return invoke(t, symbol(t, lib, "compiled_by_clang"), nil, reflect.TypeFor[int32]())[0].Int()
}

func TestLastLibraryThatResolvesWins(t *testing.T) {
	libs := compilers(t)
	if len(libs) < 2 {
		t.Skip("requires gcc and clang")
	}
	if builtByClang(t, libs[0]) == builtByClang(t, libs[1]) {
		t.Skip("requires gcc and clang to be different compilers, and macOS's gcc is clang")
	}
	for _, order := range []struct {
		libs  string
		clang int32
	}{
		{libs[0] + " " + libs[1], 1},
		{libs[1] + " " + libs[0], 0},
	} {
		var lib compilerLibrary
		if err := API.Link(api.StructureOf(&lib), order.libs, Options{}); err != nil {
			t.Fatal(err)
		}
		if got := lib.Clang(); got != order.clang {
			t.Errorf("%s: compiled_by_clang() = %d", order.libs, got)
		}
	}
}

type missingSymbol struct {
	api.Specification
	Abs func(int32) (int32, error) `call:"abs func(int)int"`
}

func TestLookupReturningNothingIsAnError(t *testing.T) {
	var lib missingSymbol
	err := API.Link(api.StructureOf(&lib), "", Options{LookupSymbol: func(string) (unsafe.Pointer, error) {
		return nil, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Abs(1); err == nil || !strings.Contains(err.Error(), "abs") {
		t.Errorf("error = %v, want one that names the symbol", err)
	}
}

func TestLinkFailureMessage(t *testing.T) {
	var lib cosLibrary
	err := API.Link(api.StructureOf(&lib), "libdoes_not_exist.so", Options{})
	if want := "library for string not available on " + runtime.GOOS; err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestStringCopySurvivesGarbageCollection(t *testing.T) {
	slow := makeFn[func(string, int32) int32](t, helper(t, "slow_strlen"), "func(&char,int)int")
	defer debug.SetGCPercent(debug.SetGCPercent(-1))

	const length = 96 // the copy, with its NUL, lands in the 112 byte noscan size class.
	var (
		stop atomic.Bool
		wg   sync.WaitGroup
		sink atomic.Pointer[[][]byte]
	)
	wg.Add(1)
	go func() { // reuses freed memory of that size class, leaving no NUL behind for strlen to find.
		defer wg.Done()
		for !stop.Load() {
			runtime.GC()
			keep := make([][]byte, 0, 4096)
			for range 4096 {
				b := make([]byte, 112)
				for i := range b {
					b[i] = 'x'
				}
				keep = append(keep, b)
			}
			sink.Store(&keep)
		}
	}()
	defer wg.Wait()
	defer stop.Store(true)
	for range 30 {
		s := strings.Repeat("a", length)
		if got := slow(s, 5); got != length {
			t.Errorf("C saw freed memory: strlen = %d, want %d", got, length)
		}
		runtime.KeepAlive(s)
	}
}

// C may free a '$' string, which is only valid for memory from C's malloc.
func TestSoldStringIsCAllocated(t *testing.T) {
	freeStr := makeFn[func(string) uint64](t, helper(t, "free_str"), "func($char)size_t")
	for _, s := range []string{"hello", strings.Repeat("a", 96), "", "terminated\x00"} {
		if got, want := freeStr(s), uint64(len(strings.TrimSuffix(s, "\x00"))); got != want {
			t.Errorf("free_str(%q) = %d, want %d", s, got, want)
		}
	}
	// the copy is released by Go when an assertion stops C from being given it.
	checked := makeFn[func(string, uint64) uint64](t, helper(t, "free_str"), "func($char[@2],size_t)size_t")
	if recovered(func() { checked("hi", 8) }) == nil {
		t.Errorf("a 2 byte string has no more than 8 bytes of capacity")
	}
	if got := checked("hello", 2); got != 5 {
		t.Errorf("free_str(hello) = %d", got)
	}
}

// C may keep a '~' string for good, after Go would have collected a copy of its own.
func TestStaticStringOutlivesTheCall(t *testing.T) {
	keep := makeFn[func(string)](t, helper(t, "keep_str"), "func(~char)void")
	kept := makeFn[func() string](t, helper(t, "kept_str"), "func()~char")
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	want := strings.Repeat("a", 96) // the copy, with its NUL, lands in the 112 byte noscan size class.
	for range 30 {
		keep(strings.Clone(want))
		runtime.GC()
		reuse := make([][]byte, 0, 4096) // takes the freed memory of that size class, if any.
		for range cap(reuse) {
			reuse = append(reuse, bytes.Repeat([]byte{'x'}, 112))
		}
		if got := kept(); got != want {
			t.Fatalf("C kept memory that Go reused: got %.16q", got)
		}
		runtime.KeepAlive(reuse)
	}
}

func TestGoMemoryIsNotGivenToC(t *testing.T) {
	jump := helper(t, "is_null")
	if _, err := Make[func([]byte) int32](jump, "func($void)int"); err == nil {
		t.Errorf("a slice C may free: want an error from Make")
	}
	if _, err := Make[func([]byte) int32](jump, "func(~char)int"); err == nil {
		t.Errorf("a slice C may keep: want an error from Make")
	}
	if _, err := Make[func(string) int32](jump, "func(+char)int"); err == nil {
		t.Errorf("a string C writes into: want an error from Make")
	}
}

func TestStringLiteralsCanBePassed(t *testing.T) {
	strlen := makeFn[func(string) uint64](t, libc(t, "strlen"), "func(&char)size_t")
	if got := strlen("hello"); got != 5 {
		t.Errorf("strlen(hello) = %d", got)
	}
	if got := strlen(""); got != 0 {
		t.Errorf(`strlen("") = %d`, got)
	}
	if got := strlen("terminated\x00"); got != 10 {
		t.Errorf("strlen(terminated) = %d", got)
	}
}

type named string
type namedPointer *byte
type namedUintptr uintptr

func TestPointerResultsFollowTheMarkerNotThePointee(t *testing.T) {
	echo := helper(t, "echo_str")
	if got := makeFn[func(string) string](t, echo, "func(&char)&char")("hello"); got != "hello" {
		t.Errorf("string result = %q", got)
	}
	if got := makeFn[func(string) named](t, echo, "func(&char)&char")("hello"); got != "hello" {
		t.Errorf("named string result = %q", got)
	}
	var b [16]byte
	in := unsafe.Pointer(&b[0])
	if got := makeFn[func(unsafe.Pointer) uintptr](t, echo, "func(&char)&char")(in); got != uintptr(in) {
		t.Errorf("uintptr result = %#x, want %#x", got, uintptr(in))
	}
	if got := makeFn[func(unsafe.Pointer) namedUintptr](t, echo, "func(&char)~char")(in); uintptr(got) != uintptr(in) {
		t.Errorf("named uintptr result = %#x, want %#x", got, uintptr(in))
	}
	if got := makeFn[func(unsafe.Pointer) unsafe.Pointer](t, echo, "func(&char)&char")(in); got != in {
		t.Errorf("unsafe.Pointer result = %p, want %p", got, in)
	}
	if got := makeFn[func(unsafe.Pointer) *byte](t, echo, "func(&char)&char")(in); unsafe.Pointer(got) != in {
		t.Errorf("*byte result = %p, want %p", got, in)
	}
	if got := makeFn[func(unsafe.Pointer) namedPointer](t, echo, "func(&char)#char")(in); unsafe.Pointer(got) != in {
		t.Errorf("named pointer result = %p, want %p", got, in)
	}
	if !makeFn[func(unsafe.Pointer) bool](t, echo, "func(&char)&char")(in) {
		t.Errorf("non-nil pointer as bool = false")
	}
	runtime.KeepAlive(&b)
}

func TestNullStringResultIsEmpty(t *testing.T) {
	if got := makeFn[func() string](t, helper(t, "null_str"), "func()&char")(); got != "" {
		t.Errorf("got %q", got)
	}
	if got := makeFn[func() string](t, helper(t, "static_str"), "func()~char")(); got != "static" {
		t.Errorf("got %q", got)
	}
}

func TestIntegerResultIsNotARune(t *testing.T) {
	if _, err := Make[func(int32) string](libc(t, "abs"), "func(int)int"); err == nil {
		t.Errorf("abs bound to a string result: want an error from Make")
	}
}

func TestSoldStringResultIsRejected(t *testing.T) {
	// copying a '$' string would leak it, as nothing is left to free it with.
	if _, err := Make[func(string) string](libc(t, "strdup"), "func(&char)$char"); err == nil {
		t.Errorf("want an error from Make")
	}
}

func TestGoFuncArgumentIsCallableFromC(t *testing.T) {
	for _, lib := range compilers(t) {
		cb := makeFn[func(func(int32) int32, int32) int32](t, symbol(t, lib, "call_cb"), "func(func(int)int,int)int")
		factor := int32(3)
		if got := cb(func(v int32) int32 { return v * factor }, 14); got != 42 {
			t.Errorf("closure called from C returned %d", got)
		}
	}
}

func TestGoFuncArgumentIsNotAGoCodePointer(t *testing.T) {
	ident := makeFn[func(func(int32) int32) uintptr](t, helper(t, "ptr_ident"), "func(func(int)int)uintptr_t")
	double := func(v int32) int32 { return v * 2 }
	code := reflect.ValueOf(double).Pointer()
	if got := ident(double); got == code || got == 0 {
		t.Errorf("C received %#x, the Go code pointer is %#x", got, code)
	}
}

func TestNilGoFuncIsNull(t *testing.T) {
	isNull := makeFn[func(func(int32) int32) int32](t, helper(t, "is_null"), "func(func(int)int)int")
	if got := isNull(nil); got != 1 {
		t.Errorf("is_null(nil) = %d", got)
	}
}

func TestSoldGoFuncOutlivesTheCall(t *testing.T) {
	keep := makeFn[func(func(int32) int32)](t, helper(t, "keep_cb"), "func($func(int)int)void")
	call := makeFn[func(int32) int32](t, helper(t, "call_kept"), "func(int)int")
	keep(func(v int32) int32 { return v + 1 })
	if got := call(41); got != 42 {
		t.Errorf("kept callback returned %d", got)
	}
}

// 840f71d passed any Go func, which only worked when it was nil, as NULL.
func TestGoFuncThatDoesNotMatchItsTagCanOnlyBeNil(t *testing.T) {
	isNull := helper(t, "is_null")
	check := func(name string, fn reflect.Value, arg reflect.Type) {
		t.Helper()
		if got := fn.Call([]reflect.Value{reflect.Zero(arg)})[0].Int(); got != 1 {
			t.Errorf("%s: is_null(nil) = %d", name, got)
		}
		never := reflect.MakeFunc(arg, func([]reflect.Value) []reflect.Value { panic("called from C") })
		if r := recovered(func() { fn.Call([]reflect.Value{never}) }); r == nil || r == "called from C" {
			t.Errorf("%s: C must not be given a function of the wrong type", name)
		}
	}
	count := makeFn[func(func(int32, int32) int32) int32](t, isNull, "func(func(int)int)int")
	check("wrong parameter count", reflect.ValueOf(count), reflect.TypeFor[func(int32, int32) int32]())
	size := makeFn[func(func(int64) int32) int32](t, isNull, "func(func(int)int)int")
	check("wrong parameter size", reflect.ValueOf(size), reflect.TypeFor[func(int64) int32]())
	result := makeFn[func(func(int32)) int32](t, isNull, "func(func(int)int)int")
	check("missing result", reflect.ValueOf(result), reflect.TypeFor[func(int32)]())
	channel := makeFn[func(func(chan int) int32) int32](t, isNull, "func(func(&void)int)int")
	check("channel parameter", reflect.ValueOf(channel), reflect.TypeFor[func(chan int) int32]())
}

func TestBackMustMatchItsTag(t *testing.T) {
	jump := helper(t, "call_cb")
	if _, err := Make[func(Back[func(int64) int32], int32) int32](jump, "func(func(int)int,int)int"); err == nil {
		t.Errorf("mismatched Back: want an error from Make")
	}
	if _, err := Make[func(Back[int], int32) int32](jump, "func(func(int)int,int)int"); err == nil {
		t.Errorf("Back of a non-function: want an error from Make")
	}
	if _, err := Make[func(Back[int]) int32](helper(t, "is_null"), "func(&void)int"); err == nil {
		t.Errorf("Back of a non-function as &void: want an error from Make")
	}
}

func TestBackCanBePassed(t *testing.T) {
	var back Back[func(int32) int32]
	back.Set(func(v int32) int32 { return v * 2 })
	defer back.Free()
	jump := helper(t, "call_cb")
	if got := makeFn[func(Back[func(int32) int32], int32) int32](t, jump, "func(func(int)int,int)int")(back, 21); got != 42 {
		t.Errorf("as func: got %d", got)
	}
	if got := makeFn[func(Back[func(int32) int32], int32) int32](t, jump, "func(&void,int)int")(back, 21); got != 42 {
		t.Errorf("as &void: got %d", got)
	}
}

func TestBackParameterTypes(t *testing.T) {
	var ints Back[func(int8, uint8, int16, uint16, int32, uint32, int, uint, float32, float64, bool) int]
	ints.Set(func(a int8, b uint8, c int16, d uint16, e int32, f uint32, g int, h uint, i float32, j float64, k bool) int {
		if a != -1 || b != 200 || c != -2 || d != 40000 || e != -3 || f != 3000000000 || g != -4 || h != 5 || i != 6 || j != 7 || !k {
			t.Errorf("got %v %v %v %v %v %v %v %v %v %v %v", a, b, c, d, e, f, g, h, i, j, k)
		}
		return 1
	})
	defer ints.Free()
	values := []any{int8(-1), uint8(200), int16(-2), uint16(40000), int32(-3), uint32(3000000000), int(-4), uint(5), float32(6), float64(7), true}
	args := make([]reflect.Value, len(values))
	for i, value := range values {
		args[i] = reflect.ValueOf(value)
	}
	if got := invoke(t, ints.pointer(), args, reflect.TypeFor[int]())[0].Int(); got != 1 {
		t.Errorf("got %d", got)
	}
}

func TestBackString(t *testing.T) {
	var back Back[func(string) int32]
	back.Set(func(s string) int32 { return int32(len(s)) })
	defer back.Free()
	got := makeFn[func(Back[func(string) int32]) string](t, helper(t, "call_str_cb"), "func(func(&char)int)&char")(back)
	if got != "ok" {
		t.Errorf("got %q", got)
	}
}

type handle struct{ V int32 }
type hidden struct{ v int32 }

func TestBackSingleFieldStruct(t *testing.T) {
	var back Back[func(handle) handle]
	back.Set(func(h handle) handle { return handle{h.V * 2} })
	defer back.Free()
	got := invoke(t, back.pointer(), []reflect.Value{reflect.ValueOf(int32(21))}, reflect.TypeFor[int32]())[0].Int()
	if got != 42 {
		t.Errorf("got %d", got)
	}
	var private Back[func(hidden) hidden]
	private.Set(func(h hidden) hidden { return hidden{h.v * 2} })
	defer private.Free()
	got = invoke(t, private.pointer(), []reflect.Value{reflect.ValueOf(int32(21))}, reflect.TypeFor[int32]())[0].Int()
	if got != 42 {
		t.Errorf("unexported field: got %d", got)
	}
}

func TestBackFuncParam(t *testing.T) {
	var back Back[func(func(int32) int32)]
	var got int32
	back.Set(func(twice func(int32) int32) { got = twice(21) })
	defer back.Free()
	makeFn[func(Back[func(func(int32) int32)])](t, helper(t, "call_with_fn"), "func(func(func(int)int))void")(back)
	if got != 42 {
		t.Errorf("function pointer from C: got %d", got)
	}
	var seenNil bool
	var null Back[func(func())]
	null.Set(func(fn func()) { seenNil = fn == nil })
	defer null.Free()
	invoke(t, null.pointer(), []reflect.Value{reflect.ValueOf(unsafe.Pointer(nil))})
	if !seenNil {
		t.Errorf("a NULL function pointer should arrive as a nil func")
	}
}

func TestBackReceivingBack(t *testing.T) {
	var inner Back[func(int32) int32]
	inner.Set(func(v int32) int32 { return v * 2 })
	defer inner.Free()
	var got unsafe.Pointer
	var outer Back[func(Back[func(int32) int32])]
	outer.Set(func(received Back[func(int32) int32]) { got = received.pointer() })
	defer outer.Free()
	invoke(t, outer.pointer(), []reflect.Value{reflect.ValueOf(inner.pointer())})
	if got != inner.pointer() {
		t.Errorf("received %p, want %p", got, inner.pointer())
	}
}

func TestBackSetRejectsWhatItCannotCall(t *testing.T) {
	for name, set := range map[string]func(){
		"channel parameter": func() { var back Back[func(chan int)]; back.Set(func(chan int) {}) },
		"map parameter":     func() { var back Back[func(map[string]int)]; back.Set(func(map[string]int) {}) },
		"variadic":          func() { var back Back[func(...int32)]; back.Set(func(...int32) {}) },
		"not a function":    func() { var back Back[int]; back.Set(1) },
	} {
		if recovered(set) == nil {
			t.Errorf("%s: Set should have panicked", name)
		}
	}
}

// 840f71d's Set accepted these, so it still doesn't panic, but there is nothing that C could call.
func TestBackSetLeavesWhatCIsUnableToCallUnset(t *testing.T) {
	var two Back[func() (int32, int32)]
	two.Set(func() (int32, int32) { return 0, 0 })
	var str Back[func() string]
	str.Set(func() string { return "" })
	var fn Back[func() func()]
	fn.Set(func() func() { return nil })
	var param Back[func(func(chan int))]
	param.Set(func(func(chan int)) {})
	for name, pointer := range map[string]unsafe.Pointer{
		"two results":   two.pointer(),
		"string result": str.pointer(),
		"func result":   fn.pointer(),
		"func param":    param.pointer(),
	} {
		if pointer != nil {
			t.Errorf("%s: a callback was made", name)
		}
	}
	if _, err := Make[func(Back[func() string]) int32](helper(t, "is_null"), "func(&void)int"); err == nil {
		t.Errorf("a Back that C cannot call: want an error from Make")
	}
}

func TestBackFreeHasAValueReceiver(t *testing.T) {
	var free interface{ Free() } = Back[func()]{}
	free.Free()
	release := Back[func()].Free
	var back Back[func()]
	back.Set(func() {})
	release(back)
	if back.pointer() != nil {
		t.Errorf("a copy still has the callback after Free")
	}
}

func TestBackFreeOfACopy(t *testing.T) {
	var back Back[func()]
	back.Set(func() {})
	other := back
	back.Free()
	other.Free()
	if other.pointer() != nil {
		t.Errorf("the copy still has the callback after Free")
	}
}

func TestBackConcurrentSet(t *testing.T) {
	const workers, each = 8, 200
	var wrong atomic.Int64
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range each {
				want := int32(w*1000 + i)
				var back Back[func() int32]
				back.Set(func() int32 { return want })
				if got := invoke(t, back.pointer(), nil, reflect.TypeFor[int32]())[0].Int(); got != int64(want) {
					wrong.Add(1)
				}
				back.Free()
			}
		}()
	}
	wg.Wait()
	if n := wrong.Load(); n > 0 {
		t.Errorf("%d callbacks ran another callback's handler", n)
	}
}

func TestBackFreeReleasesItsHandler(t *testing.T) {
	collected := make(chan struct{})
	var back Back[func() int32]
	func() {
		captured := new([64]int32)
		runtime.SetFinalizer(captured, func(*[64]int32) { close(collected) })
		back.Set(func() int32 { return captured[0] })
	}()
	back.Free()
	deadline := time.After(10 * time.Second)
	for {
		runtime.GC()
		select {
		case <-collected:
			return
		case <-deadline:
			t.Fatal("the handler is still reachable after Free")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestBackFreeTwiceOrUnset(t *testing.T) {
	var unset Back[func()]
	unset.Free()
	var back Back[func()]
	back.Set(func() {})
	back.Free()
	back.Free()
}

func TestBackSetNilIsNull(t *testing.T) {
	var back Back[func(int32) int32]
	back.Set(nil)
	isNull := makeFn[func(Back[func(int32) int32]) int32](t, helper(t, "is_null"), "func(&void)int")
	if got := isNull(back); got != 1 {
		t.Errorf("is_null(back) = %d", got)
	}
}

// Without an assertion to say what failure looks like, 840f71d's error was always nil.
func TestErrorResultIsNilWithoutAssertion(t *testing.T) {
	closeFD := makeFn[func(int32) error](t, libc(t, "close"), "func(int)int")
	if err := closeFD(-1); err != nil {
		t.Errorf("close(-1) = %v", err)
	}
	getenv := makeFn[func(string) error](t, libc(t, "getenv"), "func(&char)&char")
	if err := getenv("DOES_NOT_EXIST_ANYWHERE"); err != nil {
		t.Errorf("a NULL pointer result = %v", err)
	}
	atof := makeFn[func(string) error](t, libc(t, "atof"), "func(&char)double")
	if err := atof("1.5"); err != nil {
		t.Errorf("a double result = %v", err)
	}
	isNonZero := makeFn[func(int32) error](t, helper(t, "is_nonzero"), "func(int)bool")
	if err := isNonZero(0); err != nil {
		t.Errorf("a false result = %v", err)
	}
}

func TestErrorResultFollowsAssertion(t *testing.T) {
	closeFD := makeFn[func(int32) error](t, libc(t, "close"), "func(int)int=0")
	if err := closeFD(-1); err == nil {
		t.Errorf("assertion on the result was ignored")
	}
	abs := makeFn[func(int32) error](t, libc(t, "abs"), "func(int)int=@1")
	if err := abs(5); err != nil {
		t.Errorf("abs(5) = 5 should meet =@1: %v", err)
	}
	if err := abs(-5); err == nil {
		t.Errorf("abs(-5) != -5 should fail =@1")
	}
	nonZero := makeFn[func(int32) error](t, libc(t, "abs"), "func(int)int!=0")
	if err := nonZero(0); err == nil {
		t.Errorf("abs(0) should fail !=0")
	}
}

func TestVoidFunctionHasNoErrorToReturn(t *testing.T) {
	if _, err := Make[func(uint32) error](libc(t, "srand"), "func(unsigned_int)void"); err == nil {
		t.Errorf("want an error from Make")
	}
}

func TestErrorResultFollowsOldOrEqualAssertion(t *testing.T) {
	positive := makeFn[func(int32, int32) error](t, helper(t, "first_of"), "func(int,int)int>0=")
	if err := positive(-1, 0); err == nil {
		t.Errorf("-1 should fail >0=")
	}
	if err := positive(0, 0); err != nil {
		t.Errorf("0 should meet >0=: %v", err)
	}
}

// 840f71d checked no value assertion, so one is only checked where its failure has an error to go in.
func TestResultValueAssertionIsNotCheckedWithoutErrorResult(t *testing.T) {
	abs := makeFn[func(int32) int32](t, libc(t, "abs"), "func(int)int<10")
	if got := abs(-50); got != 50 {
		t.Errorf("abs(-50) = %d", got)
	}
	ignored := makeFn[func(int32)](t, libc(t, "abs"), "func(int)int<10")
	if r := recovered(func() { ignored(-50) }); r != nil {
		t.Errorf("panic: %v", r)
	}
}

func TestArgumentValueAssertionIsNotChecked(t *testing.T) {
	first := makeFn[func(int32, int32) int32](t, helper(t, "first_of"), "func(int>=@2,int)int")
	if got := first(2, 3); got != 2 {
		t.Errorf("got %d", got)
	}
	old := makeFn[func(int32, int32) int32](t, helper(t, "first_of"), "func(int>@2=,int)int")
	if got := old(2, 3); got != 2 {
		t.Errorf("got %d", got)
	}
	reported := makeFn[func(int32, int32) error](t, helper(t, "first_of"), "func(int=@2,int)int")
	if err := reported(2, 3); err != nil {
		t.Errorf("got %v", err)
	}
}

type failureLibrary struct {
	api.Specification
	IsNull func(unsafe.Pointer) error `call:"is_null func(&void)int=0; last_error(@1)"`
}

func TestFailureSymbolExplainsAssertion(t *testing.T) {
	var lib failureLibrary
	if err := API.Link(api.StructureOf(&lib), compilers(t)[0], Options{}); err != nil {
		t.Fatal(err)
	}
	err := lib.IsNull(nil)
	if err == nil || !strings.Contains(err.Error(), "last_error returned 7") {
		t.Errorf("error = %v", err)
	}
	var x byte
	if err := lib.IsNull(unsafe.Pointer(&x)); err != nil {
		t.Errorf("error = %v", err)
	}
}

// 840f71d never looked up the symbol after ';', so one that can't be called is left out of the error.
func TestFailureSymbolIsOptional(t *testing.T) {
	closeFD := makeFn[func(int32) error](t, libc(t, "close"), "func(int)int=0; ferror(@1)")
	if err := closeFD(-1); err == nil || strings.Contains(err.Error(), "ferror returned") {
		t.Errorf("error = %v", err)
	}
	var lib optionalFailures
	if err := API.Link(api.StructureOf(&lib), compilers(t)[0], Options{}); err != nil {
		t.Fatal(err)
	}
	if err := lib.Missing(nil); err == nil || strings.Contains(err.Error(), "does_not_exist returned") {
		t.Errorf("missing symbol: error = %v", err)
	}
	if err := lib.OutOfRange(nil); err == nil {
		t.Errorf("argument out of range: want the assertion's error")
	}
	if err := lib.Unchecked(nil); err != nil {
		t.Errorf("no assertion: error = %v", err)
	}
	if got := lib.NoError(nil); got != 1 {
		t.Errorf("no error result: got %d", got)
	}
}

type optionalFailures struct {
	api.Specification
	Missing    func(unsafe.Pointer) error `call:"is_null func(&void)int=0; does_not_exist(@1)"`
	OutOfRange func(unsafe.Pointer) error `call:"is_null func(&void)int=0; last_error(@9)"`
	Unchecked  func(unsafe.Pointer) error `call:"is_null func(&void)int; does_not_exist(@1)"`
	NoError    func(unsafe.Pointer) int32 `call:"is_null func(&void)int=0; does_not_exist(@1)"`
}

type stdioLibrary struct {
	api.Specification
	Tmpfile func() unsafe.Pointer                               `call:"tmpfile func()&FILE"`
	Fwrite  func([]byte, uint64, uint64, unsafe.Pointer) uint64 `call:"fwrite func(&void,size_t*@1,size_t,&FILE)size_t"`
	Rewind  func(unsafe.Pointer)                                `call:"rewind func(&FILE)void"`
	Fclose  func(unsafe.Pointer) int32                          `call:"fclose func(&FILE)int"`
	Fread   func([]byte, uint64, uint64, unsafe.Pointer) error  `call:"fread func(&void[@3],size_t*@1,size_t,&FILE)size_t=@3; ferror(@4)"`
}

func TestDocumentedFreadExample(t *testing.T) {
	var lib stdioLibrary
	if err := API.Link(api.StructureOf(&lib), "libc.so.6", Options{}); err != nil {
		t.Skip("requires glibc: ", err)
	}
	file := lib.Tmpfile()
	if file == nil {
		t.Fatal("tmpfile failed")
	}
	defer lib.Fclose(file)
	if got := lib.Fwrite([]byte("hello"), 1, 5, file); got != 5 {
		t.Fatalf("fwrite = %d", got)
	}
	lib.Rewind(file)
	buf := make([]byte, 16)
	if err := lib.Fread(buf, 1, 4, file); err != nil {
		t.Errorf("a full read failed: %v", err)
	}
	if string(buf[:4]) != "hell" {
		t.Errorf("got %q", buf[:4])
	}
	err := lib.Fread(buf, 1, 4, file)
	if err == nil || !strings.Contains(err.Error(), "ferror returned 0") {
		t.Errorf("a short read: %v", err)
	}
	if err := lib.Fread(buf, 2, 1, file); err == nil {
		t.Errorf("an element size that isn't the size of a byte: want an error")
	}
	if err := lib.Fread(buf, 1, 16, file); err == nil {
		t.Errorf("a capacity that isn't more than the count: want an error")
	}
}

type badSignature struct {
	api.Specification
	Abs func(string) (int32, error) `call:"abs func(&void)int"`
}

func TestLinkTurnsBadSignaturesIntoErrors(t *testing.T) {
	var lib badSignature
	if err := API.Link(api.StructureOf(&lib), "libc.so.6", Options{}); err != nil {
		t.Skip("requires glibc: ", err)
	}
	if _, err := lib.Abs("x"); err == nil {
		t.Errorf("want an error")
	}
}

func TestCapacityAssertionStopsTheCall(t *testing.T) {
	memset := makeFn[func([]byte, int32, uint64) unsafe.Pointer](t, libc(t, "memset"), "func(&void[=@3],int,size_t)&void")
	backing := make([]byte, 16)
	if recovered(func() { memset(backing[:4:4], 0x41, 12) }) == nil {
		t.Errorf("a 4 byte slice must not satisfy [=@3] with @3 = 12")
	}
	if backing[0] != 0 {
		t.Errorf("C was called: %q", backing)
	}
	memset(backing[:4:4], 0x41, 4)
	if backing[3] != 0x41 || backing[4] != 0 {
		t.Errorf("got %q", backing)
	}
}

func TestCapacityAssertionBecomesAnError(t *testing.T) {
	memset := makeFn[func([]byte, int32, uint64) error](t, libc(t, "memset"), "func(&void[=@3],int,size_t)&void")
	buf := make([]byte, 4)
	if err := memset(buf, 0x41, 12); err == nil {
		t.Errorf("want an error")
	}
	if err := memset(buf, 0x41, 4); err != nil {
		t.Errorf("got %v", err)
	}
}

func TestBareCapacityIsStrictlyGreater(t *testing.T) {
	memset := makeFn[func([]byte, int32, uint64) unsafe.Pointer](t, libc(t, "memset"), "func(&void[@3],int,size_t)&void")
	buf := make([]byte, 4)
	memset(buf, 0, 3)
	if recovered(func() { memset(buf, 0, 4) }) == nil {
		t.Errorf("[x] is documented as greater and not equal")
	}
	literal := makeFn[func([]byte, int32, uint64) unsafe.Pointer](t, libc(t, "memset"), "func(&void[8],int,size_t)&void")
	if recovered(func() { literal(buf, 0, 1) }) == nil {
		t.Errorf("a 4 byte slice has no more than 8 bytes of capacity")
	}
}

func TestCapacityOfArrayPointer(t *testing.T) {
	memset := makeFn[func(*[4]byte, int32, uint64) unsafe.Pointer](t, libc(t, "memset"), "func(&void[>=@3],int,size_t)&void")
	var buf [4]byte
	memset(&buf, 0, 4)
	if recovered(func() { memset(&buf, 0, 5) }) == nil {
		t.Errorf("[>=@3] with @3 = 5 must fail for a [4]byte")
	}
}

func TestCapacityAssertionInOldOrEqualSyntax(t *testing.T) {
	memset := makeFn[func([]byte, int32, uint64) unsafe.Pointer](t, libc(t, "memset"), "func(&void[>@3=],int,size_t)&void")
	backing := make([]byte, 16)
	memset(backing[:4:4], 0x41, 4)
	if recovered(func() { memset(backing[:4:4], 0x41, 5) }) == nil {
		t.Errorf("a 4 byte slice must not satisfy [>@3=] with @3 = 5")
	}
	if backing[4] != 0 {
		t.Errorf("C was called: %q", backing)
	}
}

// 840f71d checked no assertion at all, so one that can't be checked is still ignored.
func TestUncheckableAssertionsAreIgnored(t *testing.T) {
	memset := makeFn[func(unsafe.Pointer, int32, uint64) unsafe.Pointer](t, libc(t, "memset"), "func(&void[=@3],int,size_t)&void")
	buf := make([]byte, 4)
	memset(unsafe.Pointer(&buf[0]), 0x41, 2)
	if string(buf) != "AA\x00\x00" {
		t.Errorf("capacity of an unsafe.Pointer: got %q", buf)
	}
	memcpy := makeFn[func([]byte, []int32, uint64) unsafe.Pointer](t, libc(t, "memcpy"), "func(&void,&void:@1,size_t)&void")
	memcpy(buf, []int32{0x44434241}, 4)
	if string(buf) != "ABCD" {
		t.Errorf("same type: got %q", buf)
	}
	first := makeFn[func(int32, int32) int32](t, helper(t, "first_of"), "func(int,int?@1)int")
	if got := first(4, 5); got != 4 {
		t.Errorf("printf-style: got %d", got)
	}
	abs := makeFn[func(int32) int32](t, libc(t, "abs"), "func(int=@7)int")
	if got := abs(-3); got != 3 {
		t.Errorf("out of range: got %d", got)
	}
	constant := makeFn[func(int32) int32](t, libc(t, "abs"), "func(int=NO_SUCH_CONSTANT)int")
	if got := constant(-3); got != 3 {
		t.Errorf("unknown constant: got %d", got)
	}
	atof := makeFn[func(string) error](t, libc(t, "atof"), "func(&char)double>0=")
	if err := atof("-1"); err != nil {
		t.Errorf("assertion on a double: got %v", err)
	}
}

func TestSizeofAssertion(t *testing.T) {
	memset := makeFn[func([]uint32, int32, uint64) unsafe.Pointer](t, libc(t, "memset"), "func(&void,int,size_t*@1)&void")
	buf := make([]uint32, 4)
	memset(buf, 0, 4)
	if recovered(func() { memset(buf, 0, 8) }) == nil {
		t.Errorf("8 is not the size of a uint32")
	}
}

func TestInferredSizeof(t *testing.T) {
	memset := makeFn[func([]uint32, int32) unsafe.Pointer](t, libc(t, "memset"), "func(&void,int,-size_t*@1)&void")
	buf := []uint32{0xffffffff, 0xffffffff}
	memset(buf, 0)
	if buf[0] != 0 || buf[1] != 0xffffffff {
		t.Errorf("got %x", buf)
	}
}

func TestOverlapAssertions(t *testing.T) {
	jump := libc(t, "memcmp")
	apart := makeFn[func([]byte, []byte, uint64) int32](t, jump, "func(&void,&void~@1,size_t)int")
	together := makeFn[func([]byte, []byte, uint64) int32](t, jump, "func(&void,&void|@1,size_t)int")
	buf := make([]byte, 16)
	other := make([]byte, 16)
	apart(buf, other, 1)
	together(buf, buf[4:], 1)
	if recovered(func() { apart(buf, buf[4:], 1) }) == nil {
		t.Errorf("~@1 with overlapping buffers should fail")
	}
	if recovered(func() { together(buf, other, 1) }) == nil {
		t.Errorf("|@1 with separate buffers should fail")
	}
}

func TestLifetimeAssertion(t *testing.T) {
	strchr := makeFn[func(string, int32) *byte](t, libc(t, "strchr"), "func(&char,int)&char^@1")
	if got := strchr("hello", 'l'); got == nil || *got != 'l' {
		t.Errorf("got %v", got)
	}
	if got := strchr("hello", 'z'); got != nil {
		t.Errorf("got %v", got)
	}
	outside := makeFn[func([]byte, string) unsafe.Pointer](t, libc(t, "strstr"), "func(&char,&char)&char^@2")
	if recovered(func() { outside([]byte("hello\x00"), "ll") }) == nil {
		t.Errorf("the result points into @1, not @2")
	}
}

func TestUnsignedNarrowArguments(t *testing.T) {
	for _, lib := range compilers(t) {
		u8 := makeFn[func(uint8) int32](t, symbol(t, lib, "ident_uchar"), "func(unsigned_char)int")
		u16 := makeFn[func(uint16) int32](t, symbol(t, lib, "ident_ushort"), "func(unsigned_short)int")
		if got := u8(200); got != 200 {
			t.Errorf("%s: ident_uchar(200) = %d", filepath.Base(lib), got)
		}
		if got := u16(40000); got != 40000 {
			t.Errorf("%s: ident_ushort(40000) = %d", filepath.Base(lib), got)
		}
	}
}

func TestBoolArgument(t *testing.T) {
	fn := makeFn[func(bool) int32](t, helper(t, "ident_bool"), "func(bool)int")
	if got := fn(true); got != 11 {
		t.Errorf("true: got %d", got)
	}
	if got := fn(false); got != 22 {
		t.Errorf("false: got %d", got)
	}
}

type myInt int32
type myBool bool

func TestNamedResults(t *testing.T) {
	abs := makeFn[func(int32) myInt](t, libc(t, "abs"), "func(int)int")
	if got := abs(-5); got != 5 {
		t.Errorf("got %d", got)
	}
	flag := makeFn[func(int32) myBool](t, libc(t, "abs"), "func(int)int")
	if got := flag(-5); got != true {
		t.Errorf("got %v", got)
	}
}

func TestResultCanBeIgnored(t *testing.T) {
	makeFn[func(int32)](t, libc(t, "abs"), "func(int)int")(-5)
}

func TestVoidResult(t *testing.T) {
	makeFn[func(uint32)](t, libc(t, "srand"), "func(unsigned_int)void")(1)
	makeFn[func(uint32)](t, libc(t, "srand"), "func(unsigned_int)")(1)
}

func TestUnknownPointeeIsOpaque(t *testing.T) {
	tmpfile := makeFn[func() unsafe.Pointer](t, libc(t, "tmpfile"), "func()&FILE")
	fclose := makeFn[func(unsafe.Pointer) int32](t, libc(t, "fclose"), "func(&FILE)int")
	file := tmpfile()
	if file == nil {
		t.Fatal("tmpfile failed")
	}
	if got := fclose(file); got != 0 {
		t.Errorf("fclose = %d", got)
	}
}

func TestMakeReportsSignaturesThatCannotWork(t *testing.T) {
	jump := libc(t, "abs")
	expectError := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Errorf("%s: want an error from Make", name)
		}
	}
	_, err := Make[func(string) int32](jump, "func(not_a_c_type)int")
	expectError("unknown value type", err)
	_, err = Make[func(int32) int32](jump, "func(int)not_a_c_type")
	expectError("unknown result type", err)
	_, err = Make[func(int32) int32](jump, "int")
	expectError("not a function", err)
	_, err = Make[int](jump, "func(int)int")
	expectError("Go type is not a function", err)
	_, err = Make[func(...int32) int32](jump, "func(int)int")
	expectError("variadic Go function", err)
	_, err = Make[func(int32) (int32, int32)](jump, "func(int)int")
	expectError("two results", err)
	_, err = Make[func() int32](jump, "func(int)int")
	expectError("too few Go arguments", err)
	_, err = Make[func(int32) int32](jump, "func(int)")
	expectError("Go wants a result that C doesn't return", err)
	_, err = Make[func(chan int) int32](jump, "func(&void)int")
	expectError("unsupported pointer argument", err)
	_, err = Make[func(int) int32](jump, "func(&void)int")
	expectError("integer for a pointer to void", err)
	_, err = Make[func(float64) int32](jump, "func(&char)int")
	expectError("float for a pointer", err)
	_, err = Make[func(unsafe.Pointer) float32](jump, "func(float)float")
	expectError("pointer for a float", err)
	_, err = Make[func(string) int32](jump, "func(&void)int")
	expectError("string for &void", err)
	_, err = Make[func(float64) float32](jump, "func(void)int")
	expectError("void parameter", err)
	_, err = Make[func() map[string]int](jump, "func()int")
	expectError("unsupported result", err)
	_, err = Make[func() chan int](jump, "func()&void")
	expectError("unsupported pointer result", err)
	_, err = Make[func() fmt.Stringer](jump, "func()int")
	expectError("non-error interface", err)
	_, err = Make[func() float64](jump, "func()&void")
	expectError("float from a pointer", err)
	_, err = Make[func([]byte) int32](jump, "func(&void,-int)int")
	expectError("nothing to infer from", err)
	_, err = Make[func([]byte) int32](jump, "func(&void,-int=@7)int")
	expectError("inferred from out of range", err)
	_, err = Make[func([]byte) int32](jump, "func(&void,-int=NO_SUCH_CONSTANT)int")
	expectError("inferred from an unknown constant", err)
}

// 840f71d passed an integer for a pointer to an integer type, such as 0 for NULL.
func TestIntegerForPointerToInteger(t *testing.T) {
	ident := makeFn[func(int) uintptr](t, helper(t, "ptr_ident"), "func(&char)uintptr_t")
	for _, value := range []int{0, 5, 300, -1} {
		if got := ident(value); got != uintptr(value) {
			t.Errorf("%d arrived as %#x", value, got)
		}
	}
	if got := makeFn[func(int) uintptr](t, helper(t, "ptr_ident"), "func(&long)uintptr_t")(0x123456789); got != 0x123456789 {
		t.Errorf("&long: got %#x", got)
	}
	if got := makeFn[func(int32) uintptr](t, helper(t, "ptr_ident"), "func($int)uintptr_t")(-1); got != ^uintptr(0) {
		t.Errorf("$int: got %#x", got)
	}
	if got := makeFn[func(uint8) uintptr](t, helper(t, "ptr_ident"), "func(&unsigned_char)uintptr_t")(200); got != 200 {
		t.Errorf("&unsigned_char: got %#x", got)
	}
	isNull := makeFn[func(int) int32](t, helper(t, "is_null"), "func(&char)int")
	if got := isNull(0); got != 1 {
		t.Errorf("is_null(0) = %d", got)
	}
}

// 840f71d passed the pointer and C read as much of it as its parameter holds.
func TestPointerForIntegerParameter(t *testing.T) {
	first := makeFn[func(unsafe.Pointer, int32) int32](t, helper(t, "first_of"), "func(int,int)int")
	var b [8]byte
	in := unsafe.Pointer(&b[0])
	if got := first(in, 0); got != int32(uintptr(in)) {
		t.Errorf("got %#x, want the low half of %p", got, in)
	}
	runtime.KeepAlive(&b)
}

// 840f71d returned the address as an integer when the pointer was to an integer as wide as it.
func TestPointerResultAsInteger(t *testing.T) {
	echo := helper(t, "echo_str")
	var b [16]byte
	in := unsafe.Pointer(&b[0])
	if got := makeFn[func(unsafe.Pointer) int64](t, echo, "func(&char)&long")(in); got != int64(uintptr(in)) {
		t.Errorf("int64: got %#x, want %p", got, in)
	}
	if got := makeFn[func(unsafe.Pointer) uint64](t, echo, "func(&char)&size_t")(in); got != uint64(uintptr(in)) {
		t.Errorf("uint64: got %#x, want %p", got, in)
	}
	if got := makeFn[func(unsafe.Pointer) int32](t, echo, "func(&char)&long")(in); got != int32(uintptr(in)) {
		t.Errorf("int32: got %#x, want %p", got, in)
	}
	runtime.KeepAlive(&b)
}

func TestExtraGoArgumentsAreIgnored(t *testing.T) {
	abs := makeFn[func(int32, int32) int32](t, libc(t, "abs"), "func(int)int")
	if got := abs(-3, 9); got != 3 {
		t.Errorf("got %d", got)
	}
	last := makeFn[func(uint64, string, int32) int64](t, helper(t, "len_then_str"), "func(size_t,&char,-int=@2)long")
	if got := last(3, "ab", 9); got != 3002 {
		t.Errorf("got %d", got)
	}
}

// 840f71d gave a '-' parameter a Go argument of its own, which it ignored, so a
// Go function with one keeps working, as does one without.
func TestInferredParameterMayHaveAGoArgument(t *testing.T) {
	first := helper(t, "len_then_str")
	if got := makeFn[func(int32, string) int64](t, first, "func(-size_t=@2,&char)long")(99, "abcd"); got != 4004 {
		t.Errorf("with a Go argument for '-': got %d", got)
	}
	if got := makeFn[func(string) int64](t, first, "func(-size_t=@2,&char)long")("abcd"); got != 4004 {
		t.Errorf("without a Go argument for '-': got %d", got)
	}
	middle := helper(t, "str_len_int")
	if got := makeFn[func(string, int32, int32) int64](t, middle, "func(&char,-size_t=@1,int)long")("abc", 99, 7); got != 3003007 {
		t.Errorf("with a Go argument for '-': got %d", got)
	}
	if got := makeFn[func(string, int32) int64](t, middle, "func(&char,-size_t=@1,int)long")("abc", 7); got != 3003007 {
		t.Errorf("without a Go argument for '-': got %d", got)
	}
}

// 840f71d called a symbol whose type isn't a function as one with no parameters.
func TestNonFunctionTagIsCalledWithoutArguments(t *testing.T) {
	bumped := makeFn[func() int32](t, helper(t, "bumped"), "func()int")
	before := bumped()
	makeFn[func()](t, helper(t, "bump"), "int")()
	makeFn[func(int32)](t, helper(t, "bump"), "void")(5)
	if got := bumped() - before; got != 2 {
		t.Errorf("bump was called %d times", got)
	}
}

func TestVariadicGoFunction(t *testing.T) {
	isNull := makeFn[func(...byte) int32](t, helper(t, "is_null"), "func(&void)int")
	if got := isNull(); got != 1 {
		t.Errorf("no arguments: got %d", got)
	}
	if got := isNull(1, 2); got != 0 {
		t.Errorf("two arguments: got %d", got)
	}
}

func TestTypeAfterTheTagIsIgnored(t *testing.T) {
	for _, tag := range []string{"func(int)int)", "func(int)int, labs func(long)long"} {
		if got := makeFn[func(int32) int32](t, libc(t, "abs"), tag)(-3); got != 3 {
			t.Errorf("%s: got %d", tag, got)
		}
	}
}

func TestInferredArgument(t *testing.T) {
	snprintf := makeFn[func([]byte, string) int32](t, libc(t, "snprintf"), "func(&char,-size_t=@1,&char)int")
	buf := make([]byte, 16)
	if got := snprintf(buf, "hi"); got != 2 {
		t.Errorf("snprintf = %d", got)
	}
	if string(buf[:3]) != "hi\x00" {
		t.Errorf("buf = %q", buf)
	}
}

func TestInferredFromConstantsAndLiterals(t *testing.T) {
	memset := makeFn[func([]byte) unsafe.Pointer](t, libc(t, "memset"), "func(&void,-int=0x41,-size_t=2)&void")
	buf := make([]byte, 4)
	memset(buf)
	if string(buf) != "AA\x00\x00" {
		t.Errorf("literal: got %q", buf)
	}
	eof := makeFn[func([]byte) unsafe.Pointer](t, libc(t, "memset"), "func(&void,-int=EOF,-size_t=1)&void")
	eof(buf)
	if buf[0] != 0xff {
		t.Errorf("constant: got %q", buf)
	}
}

func TestInferredFromStringAndInteger(t *testing.T) {
	strncmp := makeFn[func(string, string) int32](t, libc(t, "strncmp"), "func(&char,&char,-size_t=@1)int")
	if got := strncmp("ab", "abc"); got != 0 {
		t.Errorf("got %d", got)
	}
	memset := makeFn[func([]byte, int32) unsafe.Pointer](t, libc(t, "memset"), "func(&void,int,-size_t=@2)&void")
	buf := make([]byte, 4)
	memset(buf, 2)
	if string(buf) != "\x02\x02\x00\x00" {
		t.Errorf("got %q", buf)
	}
}

func TestFunctionPointerResult(t *testing.T) {
	get := makeFn[func() func(int32) int32](t, helper(t, "get_twice"), "func()func(int)int")
	if got := get()(21); got != 42 {
		t.Errorf("got %d", got)
	}
	null := makeFn[func() func(int32) int32](t, helper(t, "get_null"), "func()func(int)int")
	if null() != nil {
		t.Errorf("a NULL function pointer should be a nil func")
	}
	pointer := makeFn[func() unsafe.Pointer](t, helper(t, "get_twice"), "func()func(int)int")
	if pointer() == nil {
		t.Errorf("function pointer as unsafe.Pointer is nil")
	}
	if _, err := Make[func() func(string)](helper(t, "get_twice"), "func()func(int)int"); err == nil {
		t.Errorf("mismatched function pointer: want an error from Make")
	}
}

func TestStackArgumentsPastTheDefaultBuffer(t *testing.T) {
	// last_of is variadic, and Apple's arm64 convention passes variadic
	// arguments only on the stack, where a plain call never puts the first few.
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		t.Skip("requires variadic arguments to be passed like fixed ones")
	}
	const count = 1000
	args := []reflect.Value{reflect.ValueOf(int32(count))}
	for i := range count {
		args = append(args, reflect.ValueOf(int64(i+1)))
	}
	if got := invoke(t, helper(t, "last_of"), args, reflect.TypeFor[int64]())[0].Int(); got != count {
		t.Errorf("last argument = %d, want %d", got, count)
	}
}

func TestGoFuncFloatArguments(t *testing.T) {
	add := makeFn[func(func(float64, float64) float64, float64, float64) float64](t, helper(t, "add_double"), "func(func(double,double)double,double,double)double")
	if got := add(func(a, b float64) float64 { return a + b }, 1.5, 2.25); got != 3.75 {
		t.Errorf("got %v", got)
	}
}

func TestPanicInGoFuncReachesTheGoCaller(t *testing.T) {
	cb := makeFn[func(func(int32) int32, int32) int32](t, helper(t, "call_cb"), "func(func(int)int,int)int")
	r := recovered(func() { cb(func(int32) int32 { panic("from the callback") }, 1) })
	if r != "from the callback" {
		t.Errorf("recovered %v", r)
	}
}
