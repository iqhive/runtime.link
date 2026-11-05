package ffi

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"unsafe"

	"github.com/iqhive/runtime.link/api/call/internal/cpu"
	"github.com/iqhive/runtime.link/api/call/internal/dll"
)

var helpers struct {
	once sync.Once
	dir  string
	lib  string
	err  error
}

func TestMain(m *testing.M) {
	code := m.Run()
	if helpers.dir != "" {
		os.RemoveAll(helpers.dir)
	}
	os.Exit(code)
}

func symbol(t *testing.T, lib, name string) unsafe.Pointer {
	t.Helper()
	table, err := dll.Open(lib)
	if err != nil {
		t.Skip("requires dynamic loading: ", err)
	}
	ptr, err := dll.Sym(table, name)
	if err != nil {
		t.Fatal(err)
	}
	return ptr
}

func libc(t *testing.T, name string) unsafe.Pointer { return symbol(t, "libc.so.6", name) }

func helper(t *testing.T, name string) unsafe.Pointer {
	t.Helper()
	helpers.once.Do(func() {
		if _, err := exec.LookPath("gcc"); err != nil {
			return
		}
		helpers.dir, helpers.err = os.MkdirTemp("", "ffi-helpers")
		if helpers.err != nil {
			return
		}
		helpers.lib = filepath.Join(helpers.dir, "helpers.so")
		out, err := exec.Command("gcc", "-O2", "-shared", "-fPIC", "-o", helpers.lib, "../../testdata/helpers.c").CombinedOutput()
		if err != nil {
			t.Log(string(out))
			helpers.err = err
		}
	})
	if helpers.err != nil {
		t.Fatal(helpers.err)
	}
	if helpers.lib == "" {
		t.Skip("requires a C compiler")
	}
	return symbol(t, helpers.lib, name)
}

type compiler func(reflect.Type, Type) (*cpu.Program, error)

// compiled returns a Go function for the tag with each compiler.
func compiled[T any](t *testing.T, tag string, ptr unsafe.Pointer) map[string]T {
	t.Helper()
	_, ctype, err := ParseTag(tag)
	if err != nil {
		t.Fatal(err)
	}
	functions := make(map[string]T)
	for name, compile := range map[string]compiler{"CompileForSpeed": CompileForSpeed, "CompileReliably": CompileReliably} {
		program, err := compile(reflect.TypeFor[T](), ctype)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		program.Call = ptr
		functions[name] = program.MakeFunc(reflect.TypeFor[T]()).Interface().(T)
	}
	return functions
}

func TestCompiledIntegerArguments(t *testing.T) {
	for name, sub := range compiled[func(int64, int64) int64](t, "sub2 func(long,long)long", helper(t, "sub2")) {
		if got := sub(10, 3); got != 7 {
			t.Errorf("%s: sub2(10,3) = %d", name, got)
		}
	}
}

func TestCompiledThirdIntegerArgument(t *testing.T) {
	// the third integer goes in RDX, which only the slower of the two can reach.
	for name, add := range compiled[func(int32, int32, int32) int32](t, "add3 func(int,int,int)int", helper(t, "add3")) {
		if got := add(1, 2, 3); got != 6 {
			t.Errorf("%s: add3(1,2,3) = %d", name, got)
		}
	}
}

func TestCompiledArgumentsAreWidened(t *testing.T) {
	for name, labs := range compiled[func(int32) int64](t, "labs func(long)long", libc(t, "labs")) {
		if got := labs(-5); got != 5 {
			t.Errorf("%s: labs(int32(-5)) = %d", name, got)
		}
	}
	for name, labs := range compiled[func(int8) int64](t, "labs func(long)long", libc(t, "labs")) {
		if got := labs(-5); got != 5 {
			t.Errorf("%s: labs(int8(-5)) = %d", name, got)
		}
	}
	for name, labs := range compiled[func(uint32) int64](t, "labs func(long)long", libc(t, "labs")) {
		if got := labs(math.MaxUint32); got != math.MaxUint32 {
			t.Errorf("%s: labs(MaxUint32) = %d", name, got)
		}
	}
}

func TestCompiledNarrowArgumentsArePromoted(t *testing.T) {
	for name, fn := range compiled[func(uint8) int32](t, "ident_uchar func(unsigned_char)int", helper(t, "ident_uchar")) {
		if got := fn(200); got != 200 {
			t.Errorf("%s: ident_uchar(200) = %d", name, got)
		}
	}
	for name, fn := range compiled[func(uint16) int32](t, "ident_ushort func(unsigned_short)int", helper(t, "ident_ushort")) {
		if got := fn(40000); got != 40000 {
			t.Errorf("%s: ident_ushort(40000) = %d", name, got)
		}
	}
}

func TestCompiledFloatsAreConverted(t *testing.T) {
	for name, sqrt := range compiled[func(float32) float64](t, "sqrt func(double)double", symbol(t, "libm.so.6", "sqrt")) {
		if got := sqrt(4); got != 2 {
			t.Errorf("%s: sqrt(float32(4)) = %v", name, got)
		}
	}
	for name, sqrt := range compiled[func(float64) float32](t, "sqrtf func(float)float", symbol(t, "libm.so.6", "sqrtf")) {
		if got := sqrt(4); got != 2 {
			t.Errorf("%s: sqrtf(4) = %v", name, got)
		}
	}
}

func TestCompiledFloatsPassThrough(t *testing.T) {
	for name, pow := range compiled[func(float64, float64) float64](t, "pow func(double,double)double", symbol(t, "libm.so.6", "pow")) {
		if got := pow(2, 10); got != 1024 {
			t.Errorf("%s: pow(2,10) = %v", name, got)
		}
	}
}

func TestCompiledResultsAreWidened(t *testing.T) {
	for name, abs := range compiled[func(int32) int64](t, "abs func(int)int", libc(t, "abs")) {
		if got := abs(-5); got != 5 {
			t.Errorf("%s: abs(-5) = %d", name, got)
		}
	}
}

func TestCompiledBoolOnlyLooksAtItsByte(t *testing.T) {
	for name, fn := range compiled[func(int32) bool](t, "is_nonzero func(int)bool", helper(t, "is_nonzero")) {
		if fn(0) {
			t.Errorf("%s: is_nonzero(0) = true", name)
		}
		if !fn(1) {
			t.Errorf("%s: is_nonzero(1) = false", name)
		}
	}
}

func TestCompiledIntegerResultAsBool(t *testing.T) {
	for name, fn := range compiled[func(int32) bool](t, "abs func(int)int", libc(t, "abs")) {
		if !fn(256) {
			t.Errorf("%s: abs(256) as a bool looked at one byte of an int", name)
		}
		if fn(0) {
			t.Errorf("%s: abs(0) as a bool = true", name)
		}
	}
}

func TestCompiledStrings(t *testing.T) {
	for name, strlen := range compiled[func(string) uint64](t, "strlen func(&char)size_t", libc(t, "strlen")) {
		if got := strlen("hello"); got != 5 {
			t.Errorf("%s: strlen(hello) = %d", name, got)
		}
		if got := strlen(""); got != 0 {
			t.Errorf("%s: strlen(empty) = %d", name, got)
		}
	}
}

func TestCompiledErrorComesFromTheAssertion(t *testing.T) {
	for name, closeFD := range compiled[func(int32) error](t, "close func(int)int=0", libc(t, "close")) {
		err := closeFD(-1)
		if err == nil || err.Error() != "cpu: -1" {
			t.Errorf("%s: close(-1) = %v", name, err)
		}
	}
}

func TestCompiledErrorWithoutAnAssertionIsNonZero(t *testing.T) {
	for name, abs := range compiled[func(int32) error](t, "abs func(int)int", libc(t, "abs")) {
		if err := abs(0); err != nil {
			t.Errorf("%s: abs(0) = %v", name, err)
		}
		if err := abs(-3); err == nil || err.Error() != "cpu: 3" {
			t.Errorf("%s: abs(-3) = %v", name, err)
		}
	}
}

func TestCompiledResultComparedWithAnArgument(t *testing.T) {
	for name, abs := range compiled[func(int32) error](t, "abs func(int)int=@1", libc(t, "abs")) {
		if err := abs(5); err != nil {
			t.Errorf("%s: abs(5) = %v", name, err)
		}
		if err := abs(-5); err == nil {
			t.Errorf("%s: abs(-5) should fail =@1", name)
		}
	}
}

func TestCompiledComparisons(t *testing.T) {
	jump := libc(t, "abs")
	for _, c := range []struct {
		tag  string
		in   int32
		fail bool
	}{
		{"abs func(int)int<5", 4, false},
		{"abs func(int)int<5", 5, true},
		{"abs func(int)int>5", 6, false},
		{"abs func(int)int>5", 5, true},
		{"abs func(int)int<=5", 5, false},
		{"abs func(int)int<=5", 6, true},
		{"abs func(int)int>=5", 5, false},
		{"abs func(int)int>=5", 4, true},
		{"abs func(int)int!=5", 4, false},
		{"abs func(int)int!=5", 5, true},
		{"abs func(int)int>-1", 0, false},
		{"abs func(int)int=0x10", 16, false},
	} {
		for name, fn := range compiled[func(int32) error](t, c.tag, jump) {
			if err := fn(c.in); (err != nil) != c.fail {
				t.Errorf("%s: %s with %d = %v", name, c.tag, c.in, err)
			}
		}
	}
}

func TestCompiledConstantInAssertion(t *testing.T) {
	for name, fn := range compiled[func(int32) error](t, "abs func(int)int=EXIT_FAILURE", libc(t, "abs")) {
		if err := fn(-1); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestCompiledAssertionsThatCannotBeCheckedAreErrors(t *testing.T) {
	for _, c := range []struct {
		fn  reflect.Type
		tag string
	}{
		{reflect.TypeFor[func(int32) int32](), "f func(int)int=0"},
		{reflect.TypeFor[func(int32) int32](), "f func(int=@1)int"},
		{reflect.TypeFor[func(string) error](), "f func(&char)&char^@1"},
		{reflect.TypeFor[func(int32) error](), "f func(int)int=NO_SUCH_CONSTANT"},
		{reflect.TypeFor[func([]byte) int32](), "f func(&void,-int=@1)int"},
		{reflect.TypeFor[func(int32) error](), "f func(int)int=0; g(@1)"},
		{reflect.TypeFor[func(int32) float64](), "f func(int)int"},
		{reflect.TypeFor[func(float64) int32](), "f func(int)int"},
		{reflect.TypeFor[func(int32) int32](), "f func(int)"},
		{reflect.TypeFor[func(int32) int32](), "f func(not_a_c_type)int"},
	} {
		_, ctype, err := ParseTag(c.tag)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CompileForSpeed(c.fn, ctype); err == nil {
			t.Errorf("CompileForSpeed(%s, %s): want an error", c.fn, c.tag)
		}
		if _, err := CompileReliably(c.fn, ctype); err == nil {
			t.Errorf("CompileReliably(%s, %s): want an error", c.fn, c.tag)
		}
	}
}

func TestCompiledGoFuncArgument(t *testing.T) {
	for name, call := range compiled[func(func(int32) int32, int32) int32](t, "call_cb func(func(int)int,int)int", helper(t, "call_cb")) {
		factor := int32(6)
		if got := call(func(v int32) int32 { return v * factor }, 7); got != 42 {
			t.Errorf("%s: got %d", name, got)
		}
	}
}

func TestCallbackSignatures(t *testing.T) {
	sig := newSignature(reflect.TypeFor[func() int32]())
	if sig.Returns != 'i' {
		t.Errorf("func() int32 returns %q", sig.Returns)
	}
	for _, c := range []struct {
		typ  reflect.Type
		want rune
	}{
		{reflect.TypeFor[uint16](), 'S'},
		{reflect.TypeFor[uint8](), 'C'},
		{reflect.TypeFor[bool](), 'B'},
		{reflect.TypeFor[int16](), 's'},
		{reflect.TypeFor[int8](), 'c'},
		{reflect.TypeFor[int](), 'l'},
		{reflect.TypeFor[uint](), 'L'},
		{reflect.TypeFor[float32](), 'f'},
		{reflect.TypeFor[*byte](), 'p'},
		{reflect.TypeFor[unsafe.Pointer](), 'p'},
	} {
		if got := sigRune(c.typ); got != c.want {
			t.Errorf("sigRune(%s) = %q, want %q", c.typ, got, c.want)
		}
	}
}

func TestCallbacksThatCannotBeRepresentedAreRefused(t *testing.T) {
	for _, fn := range []reflect.Type{
		reflect.TypeFor[func(chan int)](),
		reflect.TypeFor[func() string](),
		reflect.TypeFor[func() (int32, int32)](),
		reflect.TypeFor[func(...int32)](),
	} {
		if _, err := callback(fn, Type{Name: "func"}); err == nil {
			t.Errorf("%s: want an error", fn)
		}
	}
}

func TestPlatformTypes(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("these are the values for linux/amd64")
	}
	if Sizeof("long_double") != 16 {
		t.Errorf("sizeof(long double) = %d", Sizeof("long_double"))
	}
	if Const("WINT_MAX") != "4294967295" {
		t.Errorf("WINT_MAX = %s", Const("WINT_MAX"))
	}
	if Kind("char16_t") != reflect.Uint16 || Kind("char32_t") != reflect.Uint32 || Kind("wint_t") != reflect.Uint32 {
		t.Errorf("char16_t is %s, char32_t is %s, wint_t is %s", Kind("char16_t"), Kind("char32_t"), Kind("wint_t"))
	}
	if Const("FE_DFL_ENV") != "0" {
		t.Errorf("FE_DFL_ENV = %s", Const("FE_DFL_ENV"))
	}
	if Const("FLT_MIN") != "1.1754943508222875e-38" {
		t.Errorf("FLT_MIN = %s", Const("FLT_MIN"))
	}
}
