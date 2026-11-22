//go:build cgo

package call

import (
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"

	"github.com/iqhive/runtime.link/api/call/internal/ffi"
	"github.com/iqhive/runtime.link/api/call/internal/jit"
)

func TestVariableArgumentsBeginWhereTheTagSays(t *testing.T) {
	for _, c := range []struct {
		tag  string
		fn   reflect.Type
		want jit.ABI
	}{
		{"func(int,int)int", reflect.TypeFor[func(int32, int32) int32](), platform{}},
		{"func(int,long...)long", reflect.TypeFor[func(int32, int64) int64](), platform{variadic: true, fixed: 1}},
		{"func(int,long...,long,double)long", reflect.TypeFor[func(int32, int64, int64, float64) int64](), platform{variadic: true, fixed: 1}},
		{"func(long...)long", reflect.TypeFor[func(int64) int64](), platform{variadic: true}},
		// a parameter that Go has no argument for is still one that C is passed.
		{"func(-int=1,long...,long)long", reflect.TypeFor[func(int64, int64) int64](), platform{variadic: true, fixed: 1}},
		{"func(&char,-size_t=@1,&char,int...)int", reflect.TypeFor[func([]byte, string, int32) int32](), platform{variadic: true, fixed: 3}},
		// a function pointer is passed like any other pointer.
		{"func(func(int)int,int...)int", reflect.TypeFor[func(func(int32) int32, int32) int32](), platform{variadic: true, fixed: 1}},
		{"func(int)func(int,long...)long", reflect.TypeFor[func(int32) func(int32, int64) int64](), platform{}},
		// and it can be the first of the variable arguments, as the callback given to
		// curl_easy_setopt is: the dots after its type are not about its result.
		{"func(&void,int,func(&char,size_t,size_t,&void)size_t...)int", reflect.TypeFor[func(unsafe.Pointer, int32, func(unsafe.Pointer, uint64, uint64, unsafe.Pointer) uint64) int32](), platform{variadic: true, fixed: 2}},
		{"func(&void,int,func(int)...)int", reflect.TypeFor[func(unsafe.Pointer, int32, func(int32)) int32](), platform{variadic: true, fixed: 2}},
		{"func(&void,int,func(int)void...)int", reflect.TypeFor[func(unsafe.Pointer, int32, func(int32)) int32](), platform{variadic: true, fixed: 2}},
		{"func(&void,int,func(int)int...,long)int", reflect.TypeFor[func(unsafe.Pointer, int32, Back[func(int32) int32], int64) int32](), platform{variadic: true, fixed: 2}},
		{"func(&void,int,func(int)int...)int", reflect.TypeFor[func(unsafe.Pointer, int32, unsafe.Pointer) int32](), platform{variadic: true, fixed: 2}},
	} {
		_, ctype, err := ffi.ParseTag(" " + c.tag)
		if err != nil {
			t.Errorf("%s: %v", c.tag, err)
			continue
		}
		plan, err := newPlan("f", platform{}, c.fn, ctype, nil, nil)
		if err != nil {
			t.Errorf("%s: %v", c.tag, err)
			continue
		}
		if plan.callee != c.want {
			t.Errorf("%s: called as %+v, want %+v", c.tag, plan.callee, c.want)
		}
		if plan.abi != (platform{}) {
			t.Errorf("%s: the functions that it refers to would be called as %+v", c.tag, plan.abi)
		}
	}
}

func TestVariadicIntegers(t *testing.T) {
	for _, lib := range compilers(t) {
		last := makeFn[func(int32, int64, int64, int64) int64](t, symbol(t, lib, "last_of"), "func(int,long...,long,long)long")
		for n, want := range map[int32]int64{1: 11, 2: -22, 3: 1 << 40} {
			if got := last(n, 11, -22, 1<<40); got != want {
				t.Errorf("%s: argument %d of 3 = %d, want %d", filepath.Base(lib), n, got, want)
			}
		}
	}
}

// More of them than any platform has registers for.
func TestVariadicIntegersPastTheRegisters(t *testing.T) {
	for _, lib := range compilers(t) {
		last := makeFn[func(int32, int64, int64, int64, int64, int64, int64, int64, int64, int64, int64) int64](t,
			symbol(t, lib, "last_of"), "func(int,long...,long,long,long,long,long,long,long,long,long)long")
		for n := int32(1); n <= 10; n++ {
			if got := last(n, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10); got != int64(n) {
				t.Errorf("%s: argument %d of 10 = %d", filepath.Base(lib), n, got)
			}
		}
	}
}

func TestVariadicDoubles(t *testing.T) {
	for _, lib := range compilers(t) {
		sum := makeFn[func(int32, float64, float64, float64) float64](t, symbol(t, lib, "sum_doubles"), "func(int,double...,double,double)double")
		if got := sum(3, 1.5, 2.25, 4); got != 7.75 {
			t.Errorf("%s: sum of 3 = %v", filepath.Base(lib), got)
		}
		if got := sum(1, 1.5, 2.25, 4); got != 1.5 {
			t.Errorf("%s: sum of 1 = %v", filepath.Base(lib), got)
		}
		nine := makeFn[func(int32, float64, float64, float64, float64, float64, float64, float64, float64, float64) float64](t,
			symbol(t, lib, "sum_doubles"), "func(int,double...,double,double,double,double,double,double,double,double)double")
		if got := nine(9, 1, 2, 4, 8, 16, 32, 64, 128, 256); got != 511 {
			t.Errorf("%s: sum of 9 = %v", filepath.Base(lib), got)
		}
	}
}

// C passes a float among the variable arguments as a double, which is all that
// a variadic function is able to read. A named float stays a float.
func TestVariadicFloatsArePromoted(t *testing.T) {
	for _, lib := range compilers(t) {
		sum := makeFn[func(int32, float32, float32) float64](t, symbol(t, lib, "sum_doubles"), "func(int,float...,float)double")
		if got := sum(2, 1.5, 2.25); got != 3.75 {
			t.Errorf("%s: sum of 2 floats = %v", filepath.Base(lib), got)
		}
		scaled := makeFn[func(float32, int32, float32, float64) float64](t, symbol(t, lib, "scale_doubles"), "func(float,int,float...,double)double")
		if got := scaled(0.5, 2, 1.5, 2.5); got != 2 {
			t.Errorf("%s: scaled sum = %v", filepath.Base(lib), got)
		}
	}
}

func TestVariadicIntegersAndDoublesTogether(t *testing.T) {
	for _, lib := range compilers(t) {
		pairs := makeFn[func(int32, int64, float64, int64, float64) int64](t, symbol(t, lib, "sum_pairs"), "func(int,long...,double,long,double)long")
		if got := pairs(2, 1, 0.5, 2, 0.25); got != 753 {
			t.Errorf("%s: got %d", filepath.Base(lib), got)
		}
	}
}

// The last named argument is on the stack on every platform, as the first variable one has to be.
func TestVariadicAfterNamedArgumentsOnTheStack(t *testing.T) {
	for _, lib := range compilers(t) {
		nine := makeFn[func(int64, int64, int64, int64, int64, int64, int64, int64, int64, int64) int64](t,
			symbol(t, lib, "after_nine"), "func(long,long,long,long,long,long,long,long,long,long...)long")
		if got := nine(1, 1, 1, 1, 1, 1, 1, 1, 2, 3); got != 30208 {
			t.Errorf("%s: got %d", filepath.Base(lib), got)
		}
	}
}

func TestVariadicFormat(t *testing.T) {
	for _, lib := range compilers(t) {
		format := makeFn[func([]byte, string, string, int32, float64) int32](t, symbol(t, lib, "format_into"),
			"func(&char,-size_t=@1,&char,&char...,int,double)int")
		buf := make([]byte, 32)
		n := format(buf, "%s=%d/%.2f", "x", 7, 2.5)
		if got := string(buf[:max(n, 0)]); got != "x=7/2.50" {
			t.Errorf("%s: formatted %q", filepath.Base(lib), got)
		}
		// integers narrower than an int are passed as one.
		narrow := makeFn[func([]byte, string, int8, int16, uint8, uint16, bool) int32](t, symbol(t, lib, "format_into"),
			"func(&char,-size_t=@1,&char,signed_char...,short,unsigned_char,unsigned_short,bool)int")
		n = narrow(buf, "%d %d %d %d %d", -1, -2, 200, 40000, true)
		if got := string(buf[:max(n, 0)]); got != "-1 -2 200 40000 1" {
			t.Errorf("%s: formatted %q", filepath.Base(lib), got)
		}
	}
}

// Without any variable arguments, there are only the named ones to pass.
func TestVariadicFunctionCalledWithoutVariableArguments(t *testing.T) {
	format := makeFn[func([]byte, string) int32](t, helper(t, "format_into"), "func(&char,-size_t=@1,&char)int")
	buf := make([]byte, 8)
	if n := format(buf, "plain"); n != 5 || string(buf[:5]) != "plain" {
		t.Errorf("formatted %q", buf)
	}
}

func TestVariadicFunctionPointerResult(t *testing.T) {
	get := makeFn[func() func(int32, int64, int64) int64](t, helper(t, "get_last_of"), "func()func(int,long...,long)long")
	if got := get()(2, 5, 6); got != 6 {
		t.Errorf("got %d", got)
	}
}

// The variable arguments are read with va_arg wherever the platform passes
// them, which is where a function pointer that is the first of them has to be.
func TestGoFuncAsTheFirstVariableArgument(t *testing.T) {
	for _, lib := range compilers(t) {
		jump := symbol(t, lib, "call_variadic_cb")
		call := makeFn[func(int32, func(int32) int32) int32](t, jump, "func(int,func(int)int...)int")
		if got := call(20, func(v int32) int32 { return v + 1 }); got != 21 {
			t.Errorf("%s: a Go function: got %d", filepath.Base(lib), got)
		}
		var back Back[func(int32) int32]
		back.Set(func(v int32) int32 { return v * 2 })
		passed := makeFn[func(int32, Back[func(int32) int32]) int32](t, jump, "func(int,func(int)int...)int")
		if got := passed(21, back); got != 42 {
			t.Errorf("%s: a Back: got %d", filepath.Base(lib), got)
		}
		back.Free()
	}
}

// C passes a float among its variable arguments as a double, so a double is
// what a Go function that C calls with them has to take.
func TestGoFuncIsCalledWithAVariadicFloatAsADouble(t *testing.T) {
	if variadicOnTheStack {
		t.Skip("a Go function cannot be called with variable arguments here")
	}
	const tag = "func(func(int,float...)double)double"
	for _, lib := range compilers(t) {
		// double call_variadic_float_cb(double (*fn)(int, ...)) { float f = 2.5f; return fn(1, f); }
		jump := symbol(t, lib, "call_variadic_float_cb")
		call := makeFn[func(func(int32, float64) float64) float64](t, jump, tag)
		if got := call(func(n int32, f float64) float64 { return f + float64(n) }); got != 3.5 {
			t.Errorf("%s: a Go function that takes the double: got %v, want 3.5", filepath.Base(lib), got)
		}
		narrow := makeFn[func(func(int32, float32) float64) float64](t, jump, tag)
		if recovered(func() { narrow(func(int32, float32) float64 { return 0 }) }) == nil {
			t.Errorf("%s: a Go function that takes a float32, which is half of what C passes: want the call to fail", filepath.Base(lib))
		}
		var back Back[func(int32, float64) float64]
		back.Set(func(n int32, f float64) float64 { return f * 2 })
		passed := makeFn[func(Back[func(int32, float64) float64]) float64](t, jump, tag)
		if got := passed(back); got != 5 {
			t.Errorf("%s: a Back that takes the double: got %v, want 5", filepath.Base(lib), got)
		}
		back.Free()
		if _, err := Make[func(Back[func(int32, float32) float64]) float64](jump, tag); err == nil {
			t.Errorf("%s: a Back that takes a float32: want an error from Make", filepath.Base(lib))
		}
	}
	// a float that is named is still passed as one.
	if _, err := Make[func(Back[func(float32, float64) float64]) float64](helper(t, "is_null"), "func(func(float,float...)double)double"); err != nil {
		t.Errorf("a named float before a variable one: %v", err)
	}
	if _, err := Make[func(Back[func(float64, float64) float64]) float64](helper(t, "is_null"), "func(func(float,float...)double)double"); err == nil {
		t.Errorf("a double for a named float: want an error from Make")
	}
}

func TestVariadicSignaturesThatCannotWork(t *testing.T) {
	jump := helper(t, "last_of") // only the tag matters, as nothing is called.
	if _, err := Make[func(int32, ...int64) int64](jump, "func(int,long...)long"); err == nil {
		t.Errorf("a variadic Go function: want an error from Make")
	}
	if _, err := Make[func(int32) int64](jump, "func(int,long...)long"); err == nil {
		t.Errorf("no Go argument for the variable one in the tag: want an error from Make")
	}
	// C passes variable arguments where a Go function that it calls doesn't look for them.
	if !variadicOnTheStack {
		return
	}
	if _, err := Make[func(Back[func(int32, int64) int64]) int32](jump, "func(func(int,long...)long)int"); err == nil {
		t.Errorf("a Back for C to call with variable arguments: want an error from Make")
	}
	fn := makeFn[func(func(int32, int64) int64) int32](t, jump, "func(func(int,long...)long)int")
	if recovered(func() { fn(func(int32, int64) int64 { return 0 }) }) == nil {
		t.Errorf("a Go function for C to call with variable arguments: want the call to fail")
	}
}

// The dots after the result of the function mark no parameter, so it is called
// as a function without variable arguments, as it always was.
func TestDotsAfterTheResultAreNotVariableArguments(t *testing.T) {
	for _, tag := range []string{"func(int,long)long...", "func()func(int,long)long..."} {
		_, ctype, err := ffi.ParseTag(" " + tag)
		if err != nil {
			t.Errorf("%s: %v", tag, err)
			continue
		}
		if _, ok := ffi.VariadicFrom(ctype); ok {
			t.Errorf("%s: called with variable arguments", tag)
		}
	}
}

func TestStackArgumentsPastTheDefaultBuffer(t *testing.T) {
	const count = 1000
	args := []reflect.Value{reflect.ValueOf(int32(count))}
	for i := range count {
		args = append(args, reflect.ValueOf(int64(i+1)))
	}
	vals, _, err := platform{variadic: true, fixed: 1}.Call(helper(t, "last_of"), args, reflect.TypeFor[int64]())
	if err != nil {
		t.Fatal(err)
	}
	if got := vals[0].Int(); got != count {
		t.Errorf("last argument = %d, want %d", got, count)
	}
}
