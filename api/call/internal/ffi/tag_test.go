package ffi_test

import (
	"testing"

	"runtime.link/api/call/internal/ffi"
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
