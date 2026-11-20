//go:build cgo

package call

import (
	"math"
	"strings"
	"testing"
	"unsafe"

	"github.com/iqhive/runtime.link/api"
)

// A tag may name any standard C constant, which need not fit an int64.
func TestConstantsThatAreNotAnInt64(t *testing.T) {
	jump := helper(t, "ptr_ident") // uintptr_t ptr_ident(void *p) { return (uintptr_t)p; }
	for _, name := range []string{"SIZE_MAX", "UINT64_MAX", "ULONG_MAX", "ULLONG_MAX", "UINTPTR_MAX", "UINTMAX_MAX"} {
		atMost, err := Make[func(uintptr) error](jump, "func(uintptr_t)size_t<="+name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if err := atMost(math.MaxUint64); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		below, err := Make[func(uintptr) error](jump, "func(uintptr_t)size_t<"+name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if err := below(1 << 63); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if err := below(math.MaxUint64); err == nil {
			t.Errorf("2^64-1 is not less than %s", name)
		}
	}
	// the signed limits are still what they were.
	limit := makeFn[func(uintptr) error](t, jump, "func(uintptr_t)size_t<=INT_MAX")
	if err := limit(math.MaxInt32); err != nil {
		t.Errorf("got %v", err)
	}
	if err := limit(math.MaxInt32 + 1); err == nil {
		t.Errorf("2^31 is more than INT_MAX")
	}
	lowest := makeFn[func(int64, int64) error](t, helper(t, "sub2"), "func(long,long)long>LONG_MIN")
	if err := lowest(math.MinInt64, 0); err == nil {
		t.Errorf("LONG_MIN is not more than itself")
	}
}
func TestFloatingPointConstants(t *testing.T) {
	// double half(double x) { return x / 2; }
	half := makeFn[func(float64) error](t, helper(t, "half"), "func(double)double<DBL_MAX")
	if err := half(3); err != nil {
		t.Errorf("got %v", err)
	}
	if err := half(math.Inf(1)); err == nil {
		t.Errorf("an infinity is not less than DBL_MAX")
	}
	small := makeFn[func(float32) error](t, helper(t, "halff"), "func(float)float>=FLT_EPSILON")
	if err := small(1); err != nil {
		t.Errorf("got %v", err)
	}
	if err := small(1e-10); err == nil {
		t.Errorf("5e-11 is less than FLT_EPSILON")
	}
}
func TestNULLIsAConstant(t *testing.T) {
	str := makeFn[func() error](t, helper(t, "static_str"), "func()&char!=NULL")
	if err := str(); err != nil {
		t.Errorf("static_str: %v", err)
	}
	null := makeFn[func() error](t, helper(t, "null_str"), "func()&char!=NULL")
	if err := null(); err == nil {
		t.Errorf("null_str returns NULL: want an error")
	}
}

// There is no memory at NULL for anything to overlap, whatever a nil pointer points to.
func TestNilPointerOverlapsNothing(t *testing.T) {
	jump := helper(t, "both_null") // int both_null(void *a, void *b) { return a == NULL && b == NULL; }
	together := makeFn[func(*[16]byte, *[16]byte) int32](t, jump, "func(&void,&void|@1)int")
	apart := makeFn[func(*[16]byte, *[16]byte) int32](t, jump, "func(&void,&void~@1)int")
	var buf, other [16]byte
	if got := together(&buf, &buf); got != 0 {
		t.Errorf("got %d", got)
	}
	if recovered(func() { together(&buf, &other) }) == nil {
		t.Errorf("|@1 with separate arrays should fail")
	}
	if recovered(func() { together(nil, nil) }) == nil {
		t.Errorf("|@1 with two nil pointers should fail: neither has memory")
	}
	if got := apart(nil, nil); got != 1 {
		t.Errorf("~@1 with two nil pointers: got %d", got)
	}
	if recovered(func() { apart(&buf, &buf) }) == nil {
		t.Errorf("~@1 with the same array should fail")
	}
}

func TestInferredFromConstantsThatAreNotAnInt64(t *testing.T) {
	// long len_then_str(size_t n, const char *s) { return (long)n * 1000 + (long)strlen(s); }
	largest := makeFn[func(string) int64](t, helper(t, "len_then_str"), "func(-size_t=SIZE_MAX,&char)long")
	if got := largest("abc"); got != -997 {
		t.Errorf("a size_t of SIZE_MAX is -1 as a long: got %d, want -997", got)
	}
	// int bool_and_double(bool b, double d) { return (b ? 1000 : 0) + (int)d; }
	double := makeFn[func(bool) int32](t, helper(t, "bool_and_double"), "func(bool,-double=DBL_DIG)int")
	if got := double(true); got != 1015 {
		t.Errorf("an integer constant as a double: got %d", got)
	}
	float := makeFn[func() float32](t, helper(t, "halff"), "func(-float=FLT_MAX)float")
	if got := float(); got != math.MaxFloat32/2 {
		t.Errorf("a floating point constant: got %v", got)
	}
}

type explainedConstant struct {
	api.Specification
	Null func() error `call:"null_str func()&char!=NULL; is_null(NULL)"`
	Size func() error `call:"null_str func()&char!=NULL; ptr_ident(SIZE_MAX)"`
}

// The arguments of the symbol that explains a failure can be constants as well.
func TestFailureSymbolIsPassedConstants(t *testing.T) {
	var lib explainedConstant
	if err := API.Link(api.StructureOf(&lib), compilers(t)[0], Options{}); err != nil {
		t.Fatal(err)
	}
	if err := lib.Null(); err == nil || !strings.Contains(err.Error(), "is_null returned 1") {
		t.Errorf("is_null(NULL): got %v", err)
	}
	// its int result is the low half of what it was passed.
	if err := lib.Size(); err == nil || !strings.Contains(err.Error(), "ptr_ident returned -1") {
		t.Errorf("ptr_ident(SIZE_MAX): got %v", err)
	}
}

// A '-' parameter that is a bool is whether the number it is inferred from
// isn't zero, as it is in C, where Go has no conversion from a number.
func TestInferredBool(t *testing.T) {
	jump := helper(t, "ident_bool") // int ident_bool(bool b) { return b ? 11 : 22; }
	for tag, want := range map[string]int32{
		"func(-bool=true)int":  11,
		"func(-bool=false)int": 22,
		"func(-bool=1)int":     11,
		"func(-bool=0)int":     22,
	} {
		fn, err := Make[func() int32](jump, tag)
		if err != nil {
			t.Errorf("%s: %v", tag, err)
			continue
		}
		var got int32
		if failure := recovered(func() { got = fn() }); failure != nil || got != want {
			t.Errorf("%s: got %d (%v), want %d", tag, got, failure, want)
		}
	}
	// int bool_and_double(bool b, double d) { return (b ? 1000 : 0) + (int)d; }
	from := makeFn[func(float64) int32](t, helper(t, "bool_and_double"), "func(-bool=1,double)int")
	if got := from(7); got != 1007 {
		t.Errorf("got %d", got)
	}
	// the bool is 1 or 0, whatever it is inferred from.
	inferred := makeFn[func(int32) int32](t, helper(t, "ident_bool"), "func(-bool=@2,int)int")
	for value, want := range map[int32]int32{0: 22, 1: 11, 5: 11} {
		if got := inferred(value); got != want {
			t.Errorf("inferred from %d: got %d, want %d", value, got, want)
		}
	}
}

func TestInferredTypesThatCannotBePassed(t *testing.T) {
	jump := helper(t, "is_null") // only the tag matters, as nothing is called.
	for _, tag := range []string{
		"func(-void=0)int",
		"func(-long_double=0)int",
		"func(-not_a_c_type=0)int",
	} {
		var err error
		if failure := recovered(func() { _, err = Make[func() int32](jump, tag) }); failure != nil || err == nil {
			t.Errorf("%s: panic %v, error %v, want an error from Make", tag, failure, err)
		}
	}
}

// The comparison of an argument is checked before the call: it was once only
// the result's, so a tag could say what a call required of an argument and
// never have it read.
func TestArgumentComparisonIsChecked(t *testing.T) {
	// long sub2(long a, long b) { return a - b; }
	five := makeFn[func(int64, int64) error](t, helper(t, "sub2"), "func(long=5,long)long>LONG_MIN")
	if err := five(5, 0); err != nil {
		t.Errorf("got %v", err)
	}
	if err := five(3, 0); err == nil {
		t.Errorf("3 is not the 5 that the tag asks of the first argument")
	}
}

// An assertion that cannot be checked is refused when the tag is linked: it
// was once left out, so a tag could promise a capacity check that never ran.
func TestUncheckableAssertionIsRefused(t *testing.T) {
	jump := helper(t, "is_null") // only the tag matters, as nothing is called.
	_, err := Make[func(unsafe.Pointer, int) error](jump, "func(&void[=2],int)int")
	if err == nil || !strings.Contains(err.Error(), "the capacity of unsafe.Pointer is unknown") {
		t.Errorf("got %v, want a refusal of a capacity that cannot be checked", err)
	}
}
