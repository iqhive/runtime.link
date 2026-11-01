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

func TestVariadicMarksTheFunction(t *testing.T) {
	_, ctype, err := ffi.ParseTag("printf func(&char,int...)int")
	if err != nil {
		t.Fatal(err)
	}
	if !ctype.More || !ctype.Args[1].More {
		t.Errorf("function More=%v, argument More=%v", ctype.More, ctype.Args[1].More)
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
