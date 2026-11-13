package ffi_test

import (
	"testing"

	"github.com/iqhive/runtime.link/api/call/internal/ffi"
)

func TestClosureTag(t *testing.T) {
	const tag = `atexit func($func())`

	symbols, ctype, err := ffi.ParseTag(tag)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 1 || symbols[0] != "atexit" {
		t.Fatal("expected 1 symbol to be 'atexit'")
	}
	if len(ctype.Args) != 1 {
		t.Fatal("expected 1 argument")
	}
	if ctype.Args[0].Name != "func" || ctype.Args[0].Free != '$' || len(ctype.Args[0].Args) != 0 {
		t.Fatal("expected 1st argument to be mutable '$func()'")
	}
}

func TestTagParsing(t *testing.T) {
	const tag = `fread func(&void[=@3],size_t*=@1,size_t,&FILE)size_t=@3; ferror(@4)`

	symbols, ctype, err := ffi.ParseTag(tag)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 1 || symbols[0] != "fread" {
		t.Fatal("expected 1 symbol to be 'fread'")
	}
	if ctype.Func == nil || ctype.Func.Name != "size_t" {
		t.Fatal("expected type name to be function that returns 'size_t'")
	}
	if len(ctype.Args) != 4 {
		t.Fatal("expected 4 arguments")
	}
	if ctype.Args[0].Name != "void" || ctype.Args[0].Hash || ctype.Args[0].Free != '&' {
		t.Fatal("expected 1st argument to be mutable '&void'")
	}
	if !ctype.Args[0].Test.Capacity || ctype.Args[0].Test.Equality.Index != 3 {
		t.Fatal("expected 1st argument to have capacity assertion")
	}
	if ctype.Args[1].Name != "size_t" || ctype.Args[1].Free != 0 {
		t.Fatal("expected 2nd argument to be immutable 'size_t'")
	}
	if ctype.Args[1].Test.Indirect != 1 || ctype.Args[1].Test.Equality.Index != 1 {
		t.Fatal("expected 2nd argument to have indirect value assertion")
	}
	if ctype.Args[2].Name != "size_t" || ctype.Args[2].Free != 0 {
		t.Fatal("expected 3rd argument to be immutable 'size_t'")
	}
	if ctype.Args[3].Name != "FILE" || ctype.Args[3].Free != '&' {
		t.Fatal("expected 4th argument to be mutable '&FILE'")
	}
	if ctype.Func.Test.Equality.Index != 3 {
		t.Fatal("expected return value to have assertion to be equal to 3rd argument")
	}
	if ctype.Call.Name != "ferror" {
		t.Fatal("expected function to call 'ferror' on failure")
	}
	if len(ctype.Call.Args) != 1 || ctype.Call.Args[0].Index != 4 {
		t.Fatal("expected function to call 'ferror' with argument 4")
	}
}

func TestDocumentedTagsParse(t *testing.T) {
	for _, tag := range []string{
		"fread func(&void[@3],size_t*@1,size_t,&FILE)size_t<@3; ferror(@4)",
		"f func(int>=@1)",
		"f func(int<=@1)",
		"f func()int>=0",
		"f func()int<=0",
		"f func(&void[@1],int)",
		"f func(&void[8],int)",
		"f func(&char|@2,&char)",
		"f func(int=0x10)",
		"f func(int=-1)",
		"printf func(&char,int...)int",
		"f func(int...)",
		"f func(char*)",
		"f func();ferror(@1)",
		"abs func(int)int ",
		"abs func(int) int",
		"abs func(int, int)int",
		"abs func(int ,int)int",
		"abs func( int , int ) int ",
		"f func(&void[=@2],int)",
		"f func(&void[!=@2],int)",
		"f func(int!=@1)",
	} {
		if _, _, err := ffi.ParseTag(tag); err != nil {
			t.Errorf("%q: %v", tag, err)
		}
	}
}

func TestMalformedTagsAreRejected(t *testing.T) {
	for _, tag := range []string{
		"abs func(int)int xyz",
		"f func(int=)",
		"f func(int=-@1)",
		"f func(&void[@1)",
		"f func(int..)",
		"f func(int@1)",
		"f func(int",
	} {
		if _, _, err := ffi.ParseTag(tag); err == nil {
			t.Errorf("%q: expected a syntax error", tag)
		}
	}
}

func TestOrEqualAssertionsSetBothComparisons(t *testing.T) {
	_, ctype, err := ffi.ParseTag("f func(int>=@1,int<=5)")
	if err != nil {
		t.Fatal(err)
	}
	more := ctype.Args[0].Test
	if more.MoreThan.Index != 1 || more.Equality.Index != 1 || more.LessThan.Check {
		t.Errorf(">=@1 parsed as %+v", more)
	}
	less := ctype.Args[1].Test
	if less.LessThan.Value != 5 || less.Equality.Value != 5 || less.MoreThan.Check {
		t.Errorf("<=5 parsed as %+v", less)
	}
}

func TestBareCapacityMeansGreaterThan(t *testing.T) {
	_, ctype, err := ffi.ParseTag("f func(&void[@2],&void[8])")
	if err != nil {
		t.Fatal(err)
	}
	if test := ctype.Args[0].Test; !test.Capacity || test.MoreThan.Index != 2 || test.Equality.Check {
		t.Errorf("[@2] parsed as %+v", test)
	}
	if test := ctype.Args[1].Test; !test.Capacity || test.MoreThan.Value != 8 {
		t.Errorf("[8] parsed as %+v", test)
	}
}

func TestLiteralsAcceptHexAndSign(t *testing.T) {
	_, ctype, err := ffi.ParseTag("f func(int=0x10,int=-1)")
	if err != nil {
		t.Fatal(err)
	}
	if got := ctype.Args[0].Test.Equality.Value; got != 16 {
		t.Errorf("0x10 parsed as %d", got)
	}
	if got := ctype.Args[1].Test.Equality.Value; got != -1 {
		t.Errorf("-1 parsed as %d", got)
	}
}

func TestSizeofShorthandMatchesLongForm(t *testing.T) {
	_, ctype, err := ffi.ParseTag("f func(&void,size_t*@1,size_t*=@1)")
	if err != nil {
		t.Fatal(err)
	}
	short, long := ctype.Args[1].Test, ctype.Args[2].Test
	if short.Indirect != long.Indirect || short.Equality != long.Equality {
		t.Errorf("*@1 parsed as %+v, *=@1 as %+v", short, long)
	}
}

func TestOverlapMarkers(t *testing.T) {
	_, ctype, err := ffi.ParseTag("f func(&char|@2,&char~@1)")
	if err != nil {
		t.Fatal(err)
	}
	if test := ctype.Args[0].Test; test.Overlaps.Index != 2 || test.Inverted {
		t.Errorf("|@2 parsed as %+v", test)
	}
	if test := ctype.Args[1].Test; test.Overlaps.Index != 1 || !test.Inverted {
		t.Errorf("~@1 parsed as %+v", test)
	}
}

func TestVariadicMarksTheFirstVariableArgument(t *testing.T) {
	_, ctype, err := ffi.ParseTag("printf func(&char,int...,double)int")
	if err != nil {
		t.Fatal(err)
	}
	if ctype.More || ctype.Args[0].More || !ctype.Args[1].More || ctype.Args[2].More || ctype.Func.More {
		t.Errorf("function More=%v, argument More=%v %v %v, result More=%v, want it on the second argument only",
			ctype.More, ctype.Args[0].More, ctype.Args[1].More, ctype.Args[2].More, ctype.Func.More)
	}
}

// The dots come after the whole type of a function pointer, which ends with its
// result, so they are not the result's: a result cannot be a variable argument.
func TestVariadicMarkAfterAFunctionPointer(t *testing.T) {
	for _, c := range []struct {
		tag    string
		result string // of the function pointer, empty if it has none.
	}{
		{"f func(&void,int,func(&char,size_t)size_t...)int", "size_t"},
		{"f func(&void,int,func(int)...)int", ""},
		{"f func(&void,int,func(int)void...)int", "void"},
		{"f func(&void,int,func(int) int ...)int", "int"},
		{"f func(&void,int,func(int)&char...)int", "char"},
		{"f func(&void,int,func()func(int)int...)int", "func"},
	} {
		_, ctype, err := ffi.ParseTag(c.tag)
		if err != nil {
			t.Errorf("%q: %v", c.tag, err)
			continue
		}
		pointer := ctype.Args[2]
		if !pointer.More || ctype.More {
			t.Errorf("%q: function pointer More=%v, function More=%v", c.tag, pointer.More, ctype.More)
		}
		if first, ok := ffi.VariadicFrom(ctype); first != 2 || !ok {
			t.Errorf("%q: variable arguments from %d (%v), want 2", c.tag, first, ok)
		}
		if _, ok := ffi.VariadicFrom(pointer); ok {
			t.Errorf("%q: the function that is pointed to was taken to be variadic", c.tag)
		}
		switch {
		case c.result == "" && pointer.Func != nil:
			t.Errorf("%q: the function pointer has the result %+v", c.tag, pointer.Func)
		case c.result != "" && (pointer.Func == nil || pointer.Func.Name != c.result || pointer.Func.More):
			t.Errorf("%q: the function pointer has the result %+v, want %s without the dots", c.tag, pointer.Func, c.result)
		}
		if ctype.Func == nil || ctype.Func.Name != "int" {
			t.Errorf("%q: the function has the result %+v", c.tag, ctype.Func)
		}
	}
}

// An assertion on the result of a function pointer is still the result's.
func TestVariadicMarkAfterAnAssertedResult(t *testing.T) {
	_, ctype, err := ffi.ParseTag("f func(int,func(int)int>INT_MIN...)int")
	if err != nil {
		t.Fatal(err)
	}
	pointer := ctype.Args[1]
	if !pointer.More || pointer.Func == nil || pointer.Func.Test.MoreThan.Const != "INT_MIN" || pointer.Func.More {
		t.Errorf("parsed as %+v with the result %+v", pointer, pointer.Func)
	}
}

func TestInferredArgumentDoesNotShiftLaterOnes(t *testing.T) {
	_, ctype, err := ffi.ParseTag("snprintf func(&char,-size_t=@1,&char)int")
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{1, 0, 2} {
		if got := ctype.Args[i].Maps; got != want {
			t.Errorf("argument %d maps to Go argument %d, want %d", i, got, want)
		}
	}
}

func TestFailureCallAfterVoidResult(t *testing.T) {
	_, ctype, err := ffi.ParseTag("f func(&FILE);ferror(@1)")
	if err != nil {
		t.Fatal(err)
	}
	if ctype.Func != nil || ctype.Call.Name != "ferror" {
		t.Errorf("parsed as %+v", ctype)
	}
}

func TestOrEqualComesBeforeTheArgument(t *testing.T) {
	_, ctype, err := ffi.ParseTag("f func(int>=@1,int<=5,&void[>=@2])int>=0; ferror(@1)")
	if err != nil {
		t.Fatal(err)
	}
	if more := ctype.Args[0].Test; more.MoreThan.Index != 1 || more.Equality.Index != 1 || more.LessThan.Check {
		t.Errorf(">=@1 parsed as %+v", more)
	}
	if less := ctype.Args[1].Test; less.LessThan.Value != 5 || less.Equality.Value != 5 || less.MoreThan.Check {
		t.Errorf("<=5 parsed as %+v", less)
	}
	if capacity := ctype.Args[2].Test; !capacity.Capacity || capacity.MoreThan.Index != 2 || capacity.Equality.Index != 2 {
		t.Errorf("[>=@2] parsed as %+v", capacity)
	}
	if result := ctype.Func.Test; !result.MoreThan.Check || !result.Equality.Check || result.MoreThan.Value != 0 {
		t.Errorf(">=0 parsed as %+v", result)
	}
	if ctype.Call.Name != "ferror" {
		t.Errorf("parsed as %+v", ctype)
	}
}

// 840f71d wrote "or equal" with a '=' after the argument, which tags may still use.
func TestOrEqualAfterTheArgument(t *testing.T) {
	_, ctype, err := ffi.ParseTag("f func(int>@1=,int<5=,&void[>@2=])int>0=; ferror(@1)")
	if err != nil {
		t.Fatal(err)
	}
	if more := ctype.Args[0].Test; more.MoreThan.Index != 1 || more.Equality.Index != 1 || more.LessThan.Check {
		t.Errorf(">@1= parsed as %+v", more)
	}
	if less := ctype.Args[1].Test; less.LessThan.Value != 5 || less.Equality.Value != 5 || less.MoreThan.Check {
		t.Errorf("<5= parsed as %+v", less)
	}
	if capacity := ctype.Args[2].Test; !capacity.Capacity || capacity.MoreThan.Index != 2 || capacity.Equality.Index != 2 {
		t.Errorf("[>@2=] parsed as %+v", capacity)
	}
	if result := ctype.Func.Test; !result.MoreThan.Check || !result.Equality.Check || result.MoreThan.Value != 0 {
		t.Errorf(">0= parsed as %+v", result)
	}
	if ctype.Call.Name != "ferror" {
		t.Errorf("parsed as %+v", ctype)
	}
}

// 840f71d stopped at a ',' or ')' after the type and ignored the rest of the tag.
func TestTextAfterTheTypeIsIgnored(t *testing.T) {
	for _, tag := range []string{"abs func(int)int)", "abs func(int)int,x", "abs func(int)int, labs func(long)long"} {
		names, ctype, err := ffi.ParseTag(tag)
		if err != nil {
			t.Errorf("%q: %v", tag, err)
			continue
		}
		if len(names) != 1 || names[0] != "abs" || ctype.Func == nil || ctype.Func.Name != "int" || len(ctype.Args) != 1 {
			t.Errorf("%q parsed as %v %+v", tag, names, ctype)
		}
	}
}

// '@0' is the number 0, as parameters are numbered from 1.
func TestArgumentZeroIsTheNumberZero(t *testing.T) {
	_, ctype, err := ffi.ParseTag("f func(int=@0,int)int")
	if err != nil {
		t.Fatal(err)
	}
	if eq := ctype.Args[0].Test.Equality; !eq.Check || eq.Index != 0 || eq.Value != 0 {
		t.Errorf("=@0 parsed as %+v", eq)
	}
}

// The dots after the result of the function, which marks no parameter,
// leave it a function without variable arguments, as they always did.
func TestDotsAfterTheResultMarkNoParameter(t *testing.T) {
	for _, tag := range []string{"f func(&char,int)int...", "f func()func(int)int..."} {
		_, ctype, err := ffi.ParseTag(tag)
		if err != nil {
			t.Errorf("%q: %v", tag, err)
			continue
		}
		if _, ok := ffi.VariadicFrom(ctype); ok {
			t.Errorf("%q: taken to have variable arguments", tag)
		}
	}
}

// A function type among the parameters has variable arguments of its own, which
// says nothing about how the function that takes it is called.
func TestVariadicFunctionPointerDoesNotMarkTheFunction(t *testing.T) {
	_, ctype, err := ffi.ParseTag("f func(func(&char,int...)int,int)int")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ffi.VariadicFrom(ctype); ok || ctype.More || ctype.Args[0].More {
		t.Errorf("a function that takes a variadic function was marked as variadic itself")
	}
	if first, ok := ffi.VariadicFrom(ctype.Args[0]); first != 1 || !ok || !ctype.Args[0].Args[1].More {
		t.Errorf("the function pointer was not marked as variadic: %+v", ctype.Args[0])
	}
}

func TestVariadicFrom(t *testing.T) {
	for _, c := range []struct {
		tag   string
		first int
		ok    bool
	}{
		{"f func(int)int", 0, false},
		{"f func()", 0, false},
		{"f func(&char,int...)int", 1, true},
		{"f func(&char,int...,double,&char)int", 1, true},
		{"f func(int...)", 0, true},
		{"f func(&char,-size_t=@1,&char,long...)int", 3, true},
		{"f func(&char,int,long...,long...)int", 2, true},
		{"f func(func(int...)int,int)int", 0, false},
		{"f func(func(int...)int,int...)int", 1, true},
		{"f func(int)func(&char,int...)int", 0, false},
		{"f func(&void,int,func(&char)size_t...)int", 2, true},
		{"f func(func(int)...,int)int", 0, true},
		{"f func(int,func(int,long...)long...)int", 1, true},
	} {
		_, ctype, err := ffi.ParseTag(c.tag)
		if err != nil {
			t.Errorf("%q: %v", c.tag, err)
			continue
		}
		if first, ok := ffi.VariadicFrom(ctype); first != c.first || ok != c.ok {
			t.Errorf("%q: variable arguments from %d (%v), want %d (%v)", c.tag, first, ok, c.first, c.ok)
		}
	}
}
