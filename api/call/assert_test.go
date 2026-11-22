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

// The size that a capacity is compared with is the one that C is passed. A Go
// int of -1 is less than every capacity, and the size_t that it is passed as is
// more than any buffer has.
func TestCapacityIsComparedWithTheSizeThatCIsPassed(t *testing.T) {
	// size_t strnlen(const char *s, size_t maxlen), which stops at the NUL.
	strnlen := makeFn[func([]byte, int) uint64](t, libc(t, "strnlen"), "func(&char[>=@2],size_t)size_t")
	buf := []byte{'a', 'b', 'c', 0}
	if got := strnlen(buf, 4); got != 3 {
		t.Errorf("strnlen(buf, 4) = %d", got)
	}
	if recovered(func() { strnlen(buf, 5) }) == nil {
		t.Errorf("a buffer of 4 bytes has no room for 5")
	}
	if recovered(func() { strnlen(buf, -1) }) == nil {
		t.Errorf("a size of -1 is passed as SIZE_MAX, which a buffer of 4 bytes has no room for")
	}
	// int null_with_room(void *p, size_t n), which is 0 for a buffer.
	unsigned := makeFn[func([]byte, int) int32](t, helper(t, "null_with_room"), "func(&void[>=@2],unsigned_int)int")
	if recovered(func() { unsigned(buf, -1) }) == nil {
		t.Errorf("a size of -1 is passed as UINT_MAX, which a buffer of 4 bytes has no room for")
	}
	// a Go integer that is wider than the parameter is passed as what is left of it.
	narrow := makeFn[func([]byte, int64) int32](t, helper(t, "null_with_room"), "func(&void[>=@2],int)int")
	if failure := recovered(func() { narrow(buf, 1<<32|2) }); failure != nil {
		t.Errorf("a size that is passed as 2: %v", failure)
	}
	if recovered(func() { narrow(buf, -1<<32|5) }) == nil {
		t.Errorf("a negative size that is passed as 5, which a buffer of 4 bytes has no room for")
	}
	// the size that a '-' parameter is given is the one that was inferred.
	inferred := makeFn[func([]byte, []byte) int32](t, helper(t, "null_with_room"), "func(&void[>=@2],-size_t=@3,&void)int")
	if failure := recovered(func() { inferred(buf, buf[:2]) }); failure != nil {
		t.Errorf("a size of 2 for a buffer of 4 bytes: %v", failure)
	}
	if recovered(func() { inferred(buf[:2:2], buf) }) == nil {
		t.Errorf("a buffer of 2 bytes has no room for 4")
	}
}

// The comparison that is written on a number is of the number that C is passed,
// as is one that refers to it with '@n'.
func TestArgumentComparisonIsOfTheValueThatCIsPassed(t *testing.T) {
	// int first_of(int a, int b) { return a; }
	positive := makeFn[func(int64, int64) int32](t, helper(t, "first_of"), "func(int>0,int)int")
	if got := positive(7, 0); got != 7 {
		t.Errorf("first_of(7, 0) = %d", got)
	}
	if recovered(func() { positive(0, 0) }) == nil {
		t.Errorf("0 is not more than 0")
	}
	if recovered(func() { positive(1<<32, 0) }) == nil {
		t.Errorf("1<<32 is passed as an int of 0, which is not more than 0")
	}
	if recovered(func() { positive(0x1ffffffff, 0) }) == nil {
		t.Errorf("0x1ffffffff is passed as an int of -1, which is not more than 0")
	}
	var got int32
	if failure := recovered(func() { got = positive(1<<32|3, 0) }); failure != nil || got != 3 {
		t.Errorf("1<<32|3 is passed as an int of 3: got %d (%v)", got, failure)
	}
	atLeast := makeFn[func(int64, int64) int32](t, helper(t, "first_of"), "func(int>=@2,int)int")
	if recovered(func() { atLeast(1<<32, 5) }) == nil {
		t.Errorf("1<<32 is passed as an int of 0, which is less than 5")
	}
	if failure := recovered(func() { atLeast(5, 1<<32) }); failure != nil {
		t.Errorf("5 is at least the 0 that 1<<32 is passed as: %v", failure)
	}
	small := makeFn[func(int, int32) int32](t, helper(t, "first_of"), "func(unsigned_int<10,int)int")
	if recovered(func() { small(-1, 0) }) == nil {
		t.Errorf("-1 is passed as UINT_MAX, which is not less than 10")
	}
	// float halff(float x) { return x / 2; }
	single := makeFn[func(float64) float32](t, helper(t, "halff"), "func(float=16777216)float")
	var half float32
	if failure := recovered(func() { half = single(16777217) }); failure != nil || half != 8388608 {
		t.Errorf("2^24+1 is passed as a float of 2^24: got %v (%v)", half, failure)
	}
	// a number of the type that C takes is compared as it always was.
	same := makeFn[func(int32, int32) int32](t, helper(t, "first_of"), "func(int>0,int)int")
	if got := same(7, 0); got != 7 {
		t.Errorf("first_of(7, 0) = %d", got)
	}
	if recovered(func() { same(-7, 0) }) == nil {
		t.Errorf("-7 is not more than 0")
	}
}

// A comparison that is written on a pointer to an array is of the length of the
// array, and a nil pointer has none: '!=NULL' is how a tag keeps NULL from C.
func TestComparisonOnAPointerToAnArray(t *testing.T) {
	// int is_null(void *p) { return p == NULL; }
	notNull := makeFn[func(*[4]byte) int32](t, helper(t, "is_null"), "func(&void!=NULL)int")
	buf := new([4]byte)
	if got := notNull(buf); got != 0 {
		t.Errorf("is_null(buf) = %d", got)
	}
	if recovered(func() { notNull(nil) }) == nil {
		t.Errorf("a nil pointer is NULL, which '!=NULL' rules out")
	}
	// int null_with_room(void *p, size_t n) { return p == NULL && n > 0; }
	atLeast := makeFn[func(*[4]byte, int) int32](t, helper(t, "null_with_room"), "func(&void>=@2,size_t)int")
	if failure := recovered(func() { atLeast(buf, 4) }); failure != nil {
		t.Errorf("an array of 4 is at least 4: %v", failure)
	}
	if recovered(func() { atLeast(buf, 5) }) == nil {
		t.Errorf("an array of 4 is less than 5")
	}
	if recovered(func() { atLeast(nil, 1) }) == nil {
		t.Errorf("a nil pointer has no array, which is less than 1")
	}
	isNull := makeFn[func(*[4]byte) int32](t, helper(t, "is_null"), "func(&void=NULL)int")
	if got := isNull(nil); got != 1 {
		t.Errorf("is_null(nil) = %d", got)
	}
	if recovered(func() { isNull(buf) }) == nil {
		t.Errorf("a pointer to an array is not NULL")
	}
}

// A nil pointer has room for nothing, whatever its type says. Counted by its
// type, NULL passed every check of the size that C was given with it.
func TestNilPointerHasRoomForNothing(t *testing.T) {
	jump := helper(t, "null_with_room") // int null_with_room(void *p, size_t n) { return p == NULL && n > 0; }
	buf := new([4]byte)
	passes := func(name string, call func()) {
		t.Helper()
		if failure := recovered(call); failure != nil {
			t.Errorf("%s: %v", name, failure)
		}
	}
	stopped := func(name string, call func()) {
		t.Helper()
		if recovered(call) == nil {
			t.Errorf("%s: want the assertion to stop the call", name)
		}
	}
	// a size that refers to the pointer with '@n'.
	atMost := makeFn[func(*[4]byte, int) int32](t, jump, "func(&void,size_t<=@1)int")
	stopped("NULL with a size of 4", func() { atMost(nil, 4) })
	stopped("NULL with a size of 1", func() { atMost(nil, 1) })
	passes("NULL with a size of 0", func() { atMost(nil, 0) })
	passes("an array of 4 with a size of 4", func() { atMost(buf, 4) })
	stopped("an array of 4 with a size of 5", func() { atMost(buf, 5) })
	below := makeFn[func(*[4]byte, int) int32](t, jump, "func(&void,size_t<@1)int")
	stopped("NULL with a size of 1 that must be less", func() { below(nil, 1) })
	passes("NULL with a size of 0 that must be less", func() { below(nil, 0) })
	passes("an array of 4 with a size of 3 that must be less", func() { below(buf, 3) })
	stopped("an array of 4 with a size of 4 that must be less", func() { below(buf, 4) })
	// the capacity of the pointer.
	enough := makeFn[func(*[4]byte, int) int32](t, jump, "func(&void[>=@2],size_t)int")
	stopped("the capacity of NULL for a size of 1", func() { enough(nil, 1) })
	passes("the capacity of NULL for a size of 0", func() { enough(nil, 0) })
	passes("the capacity of an array of 4 for a size of 4", func() { enough(buf, 4) })
	stopped("the capacity of an array of 4 for a size of 5", func() { enough(buf, 5) })
	more := makeFn[func(*[4]byte, int) int32](t, jump, "func(&void[@2],size_t)int")
	stopped("more capacity than a size of 1 for NULL", func() { more(nil, 1) })
	passes("more capacity than a size of 0 for NULL", func() { more(nil, 0) })
	passes("more capacity than a size of 3 for an array of 4", func() { more(buf, 3) })
	stopped("more capacity than a size of 4 for an array of 4", func() { more(buf, 4) })
	literal := makeFn[func(*[4]byte, int) int32](t, jump, "func(&void[2],size_t)int")
	stopped("more capacity than 2 for NULL", func() { literal(nil, 0) })
	passes("more capacity than 2 for an array of 4", func() { literal(buf, 0) })
	// a pointer to one value has room for one.
	single := makeFn[func(*int32, int) int32](t, jump, "func(&void[>=@2],size_t)int")
	stopped("the capacity of a nil pointer for a size of 1", func() { single(nil, 1) })
	passes("the capacity of a nil pointer for a size of 0", func() { single(nil, 0) })
	passes("the capacity of a pointer for a size of 1", func() { single(new(int32), 1) })
	stopped("the capacity of a pointer for a size of 2", func() { single(new(int32), 2) })
	// its address is still what '@n' is for a pointer to one value.
	address := makeFn[func(*int32, int) int32](t, jump, "func(&void,size_t<=@1)int")
	passes("a size that is less than an address", func() { address(new(int32), 4) })
}

// A slice and a string are counted as they were: a slice by its capacity for
// '[x]' and by its length for '@n', and a string by its length for both.
func TestRoomOfASliceAndAStringIsUnchanged(t *testing.T) {
	jump := helper(t, "null_with_room") // int null_with_room(void *p, size_t n) { return p == NULL && n > 0; }
	enough := makeFn[func([]byte, int) int32](t, jump, "func(&void[>=@2],size_t)int")
	if failure := recovered(func() { enough(nil, 0) }); failure != nil {
		t.Errorf("a nil slice with a size of 0: %v", failure)
	}
	if recovered(func() { enough(nil, 1) }) == nil {
		t.Errorf("a nil slice has no room for 1")
	}
	if failure := recovered(func() { enough(make([]byte, 2, 4), 4) }); failure != nil {
		t.Errorf("a slice with a capacity of 4 and a size of 4: %v", failure)
	}
	if recovered(func() { enough(make([]byte, 2, 4), 5) }) == nil {
		t.Errorf("a slice with a capacity of 4 has no room for 5")
	}
	// more room than the size is what the bare form asks, which an empty slice never had.
	more := makeFn[func([]byte, int) int32](t, jump, "func(&void[@2],size_t)int")
	if recovered(func() { more(nil, 0) }) == nil {
		t.Errorf("a nil slice has no more room than 0")
	}
	length := makeFn[func([]byte, int) int32](t, jump, "func(&void,size_t<=@1)int")
	if failure := recovered(func() { length(make([]byte, 2, 8), 2) }); failure != nil {
		t.Errorf("a size of 2 for a slice of 2: %v", failure)
	}
	if recovered(func() { length(make([]byte, 2, 8), 3) }) == nil {
		t.Errorf("a size of 3 is more than the length of a slice of 2")
	}
	text := makeFn[func(string, int) int32](t, jump, "func(&char[>=@2],size_t)int")
	if failure := recovered(func() { text("abc", 3) }); failure != nil {
		t.Errorf("a string of 3 with a size of 3: %v", failure)
	}
	if recovered(func() { text("abc", 4) }) == nil {
		t.Errorf("a string of 3 has no room for 4")
	}
	if failure := recovered(func() { text("", 0) }); failure != nil {
		t.Errorf("an empty string with a size of 0: %v", failure)
	}
}

// A comparison that is written on a slice or a string is of its length, and
// on any other pointer of its address.
func TestComparisonOnASliceOrAStringIsOfItsLength(t *testing.T) {
	jump := helper(t, "is_null") // int is_null(void *p) { return p == NULL; }
	text := makeFn[func(string) int32](t, jump, "func(&char!=0)int")
	if got := text("a"); got != 0 {
		t.Errorf("is_null of a string = %d", got)
	}
	if recovered(func() { text("") }) == nil {
		t.Errorf("an empty string has a length of 0")
	}
	bytes := makeFn[func([]byte) int32](t, jump, "func(&void>=4)int")
	if got := bytes(make([]byte, 4)); got != 0 {
		t.Errorf("is_null of a slice = %d", got)
	}
	if recovered(func() { bytes(make([]byte, 3, 8)) }) == nil {
		t.Errorf("a slice of 3 bytes has a length that is less than 4")
	}
	pointer := makeFn[func(*int32) int32](t, jump, "func(&void!=NULL)int")
	if got := pointer(new(int32)); got != 0 {
		t.Errorf("is_null of a pointer = %d", got)
	}
	if recovered(func() { pointer(nil) }) == nil {
		t.Errorf("a nil pointer is NULL")
	}
}

type twoAssertions struct {
	api.Specification
	Ranged func(int32, int32) error `call:"first_of func(int,int)int>0<5"`
}

// A type takes one assertion. A second one after the result of the function
// was read as one on the function itself, which nothing checked: a result that
// failed it was no error.
func TestSecondAssertionAfterTheResultIsChecked(t *testing.T) {
	jump := helper(t, "first_of") // int first_of(int a, int b) { return a; }
	ranged := makeFn[func(int32, int32) error](t, jump, "func(int,int)int>0<5")
	if err := ranged(3, 0); err != nil {
		t.Errorf("3 is more than 0 and less than 5: %v", err)
	}
	if err := ranged(0, 0); err == nil {
		t.Errorf("0 is not more than 0")
	}
	if err := ranged(7, 0); err == nil || !strings.Contains(err.Error(), "int 7 fails the assertion in its tag when compared with 5") {
		t.Errorf("7 is not less than 5: got %v", err)
	}
	other := makeFn[func(int32, int32) error](t, jump, "func(int,int)int>0!=@2")
	if err := other(3, 4); err != nil {
		t.Errorf("3 is more than 0 and is not 4: %v", err)
	}
	if err := other(3, 3); err == nil {
		t.Errorf("3 is the 3 that '!=@2' rules out")
	}
	var lib twoAssertions
	if err := API.Link(api.StructureOf(&lib), compilers(t)[0], Options{}); err != nil {
		t.Fatal(err)
	}
	if err := lib.Ranged(3, 0); err != nil {
		t.Errorf("linked: 3 is more than 0 and less than 5: %v", err)
	}
	if err := lib.Ranged(7, 0); err == nil {
		t.Errorf("linked: 7 is not less than 5")
	}
	// a second assertion that cannot be checked is refused, as a first one is.
	for _, tag := range []string{"func(int,int)int>0<@7", "func(int,int)int>0<NO_SUCH_CONSTANT", "func(int,int)int>0[<5]"} {
		if _, err := Make[func(int32, int32) error](jump, tag); err == nil {
			t.Errorf("%s: want an error from Make", tag)
		}
	}
	// a third one is one more than can be written.
	if _, err := Make[func(int32, int32) error](jump, "func(int,int)int>0<5!=3"); err == nil {
		t.Errorf("three assertions: want an error from Make")
	}
	// without an error result neither comparison is made, as one is not.
	plain := makeFn[func(int32, int32) int32](t, jump, "func(int,int)int>0<5")
	if got := plain(7, 0); got != 7 {
		t.Errorf("first_of(7, 0) = %d", got)
	}
	// one assertion is checked as it was, and the dots that mark no parameter are still ignored.
	for _, tag := range []string{"func(int,int)int>0", "func(int,int)int>false..."} {
		ranged := makeFn[func(int32, int32) error](t, jump, tag)
		if err := ranged(3, 0); err != nil {
			t.Errorf("%s: got %v", tag, err)
		}
		if err := ranged(0, 0); err == nil {
			t.Errorf("%s: 0 is not more than 0", tag)
		}
	}
}

// No call sees what C passes to a Go function, nor what the function returns
// to C, so an assertion in its type cannot be checked.
func TestAssertionInACallbackTypeIsRefused(t *testing.T) {
	jump := helper(t, "call_cb") // int call_cb(int (*fn)(int), int v) { return fn(v); }
	refused := func(name string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "in the type of a function that C calls") {
			t.Errorf("%s: got %v, want a refusal of the assertion", name, err)
		}
	}
	_, err := Make[func(func(int32) int32, int32) int32](jump, "func(func(int>0)int,int)int")
	refused("on a parameter", err)
	_, err = Make[func(func(int32) int32, int32) int32](jump, "func(func(int)int=5,int)int")
	refused("on the result", err)
	_, err = Make[func(Back[func(int32) int32], int32) int32](jump, "func(func(int>0)int,int)int")
	refused("for a Back", err)
	_, err = Make[func(unsafe.Pointer, int32) int32](jump, "func(func(int>0)int,int)int")
	refused("for a pointer", err)
	_, err = Make[func(func([]byte, int) int32, int32) int32](jump, "func(func(&void[>=@2],size_t)int,int)int")
	refused("on a capacity", err)
	_, err = Make[func(func(func(int32) int32) int32) int32](helper(t, "call_with_twice"), "func(func(func(int>0)int)int)int")
	refused("in a function pointer that it is handed", err)
	// what only documents intent is left out, here as anywhere.
	if _, err := Make[func(func(int32, int32) int32, int32) int32](jump, "func(func(int,int?@1)int,int)int"); err != nil {
		t.Errorf("a printf-style assertion: %v", err)
	}
	// the assertions of a function pointer that is returned are checked by the calls that Go makes.
	get := makeFn[func() func(int32) int32](t, helper(t, "get_twice"), "func()func(int>0)int")
	if got := get()(21); got != 42 {
		t.Errorf("twice(21) = %d", got)
	}
	if recovered(func() { get()(0) }) == nil {
		t.Errorf("0 is not more than 0")
	}
}

// Without an error result the comparison of a return value is not made, but
// one that could never be made is refused, as it is with an error result.
func TestResultComparisonThatCannotBeMadeIsRefused(t *testing.T) {
	jump := helper(t, "upper") // int upper(int c)
	for _, tag := range []string{"func(int)int=@7", "func(int)int=NO_SUCH_CONSTANT"} {
		if _, err := Make[func(int32) error](jump, tag); err == nil {
			t.Errorf("%s with an error result: want an error from Make", tag)
		}
		if _, err := Make[func(int32) int32](jump, tag); err == nil {
			t.Errorf("%s with the result: want an error from Make", tag)
		}
		if _, err := Make[func(int32)](jump, tag); err == nil {
			t.Errorf("%s with no result: want an error from Make", tag)
		}
	}
	// one that could be made is still left out.
	upper := makeFn[func(int32) int32](t, jump, "func(int)int=@1")
	if got := upper('a'); got != 'A' {
		t.Errorf("upper('a') = %q", rune(got))
	}
}

// A string that ends in a NUL is passed where it is for a '#' parameter, so its
// memory is its own bytes. Only a copy has a NUL after it.
func TestMemoryOfAStringThatIsPassedAsItIs(t *testing.T) {
	jump := helper(t, "both_null") // int both_null(void *a, void *b)
	whole := string([]byte("abc\x00wxyz\x00"))
	first, second := whole[:4], whole[4:] // both end in a NUL, and share no byte.
	together := makeFn[func(string, string) int32](t, jump, "func(&#char|@2,&#char)int")
	apart := makeFn[func(string, string) int32](t, jump, "func(&#char!|@2,&#char)int")
	if failure := recovered(func() { together(first, first) }); failure != nil {
		t.Errorf("a string overlaps itself: %v", failure)
	}
	if failure := recovered(func() { together(whole, second) }); failure != nil {
		t.Errorf("a string overlaps the end of itself: %v", failure)
	}
	if recovered(func() { apart(first, first) }) == nil {
		t.Errorf("a string overlaps itself, which '!|@2' rules out")
	}
	if failure := recovered(func() { apart(first, second) }); failure != nil {
		t.Errorf("two strings that share no byte: %v", failure)
	}
	if recovered(func() { together(first, second) }) == nil {
		t.Errorf("two strings that share no byte do not overlap")
	}
	// the byte after such a string is not inside of it.
	inside := makeFn[func(string, string) int32](t, jump, "func(&#char,&#char^@1)int")
	if failure := recovered(func() { inside(first, first[1:]) }); failure != nil {
		t.Errorf("the rest of a string is inside of it: %v", failure)
	}
	if recovered(func() { inside(first, second) }) == nil {
		t.Errorf("the string after another one is outside of it")
	}
	// a copy is a byte longer than the string, for the NUL that C is passed with it.
	copied := makeFn[func(string, int32) unsafe.Pointer](t, libc(t, "strchr"), "func(&char,int)&char^@1")
	if failure := recovered(func() { copied("hello", 0) }); failure != nil {
		t.Errorf("the NUL of a copy is inside of it: %v", failure)
	}
	asItIs := makeFn[func(string, int32) unsafe.Pointer](t, libc(t, "strchr"), "func(&#char,int)&char^@1")
	if failure := recovered(func() { asItIs("hello\x00", 0) }); failure != nil {
		t.Errorf("the NUL of a string is inside of it: %v", failure)
	}
}

// '!' applies to '*@n' as it does to every other assertion.
func TestNegatedSizeofAssertion(t *testing.T) {
	jump := helper(t, "null_with_room") // int null_with_room(void *p, size_t n) { return p == NULL && n > 0; }
	sized := makeFn[func([]int32, int) int32](t, jump, "func(&void,size_t*@1)int")
	if failure := recovered(func() { sized(nil, 4) }); failure != nil {
		t.Errorf("4 is the size of an int32: %v", failure)
	}
	other := makeFn[func([]int32, int) int32](t, jump, "func(&void,size_t!*@1)int")
	if recovered(func() { other(nil, 4) }) == nil {
		t.Errorf("4 is the size of an int32, which '!*@1' rules out")
	}
	if failure := recovered(func() { other(nil, 1) }); failure != nil {
		t.Errorf("1 is not the size of an int32: %v", failure)
	}
}
