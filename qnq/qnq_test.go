package qnq_test

import (
	"context"
	"reflect"
	"testing"

	"runtime.link/qnq"
)

func TestStructure(t *testing.T) {
	var ctx = context.Background()
	var Example struct {
		_ qnq.Documentation `
			This is an example runtime.link structure.`
		HelloWorld func() string `tag:"value"
			returns "Hello World"`
	}
	Example.HelloWorld = func() string {
		return "Hello World"
	}
	structure := qnq.StructureOf(&Example)
	if structure.Docs != "This is an example runtime.link structure." {
		t.Errorf("got %q, want %q", structure.Docs, "is an example runtime.link structure.")
	}
	if len(structure.Functions) != 1 {
		t.Errorf("got %d functions, want %d", len(structure.Functions), 1)
	}
	hello := structure.Functions[0]
	if hello.Name != "HelloWorld" {
		t.Errorf("got %q, want %q", structure.Functions[0].Name, "HelloWorld")
	}
	if hello.Tags.Get("tag") != "value" {
		t.Errorf("got %q, want %q", structure.Functions[0].Tags.Get("tag"), "value")
	}
	if hello.Docs != "returns \"Hello World\"" {
		t.Errorf("got %q, want %q", structure.Functions[0].Docs, "returns \"Hello World\"")
	}
	if vals, _ := hello.Call(ctx, []reflect.Value{}); vals[0].String() != "Hello World" {
		t.Errorf("got %q, want %q", vals, "Hello World")
	}
	var ran bool
	var old = hello.Copy()
	var wrap = func() string {
		ran = true
		s, _ := old.Call(ctx, []reflect.Value{})
		return s[0].String()
	}
	hello.Make(wrap)
	if vals, _ := hello.Call(ctx, []reflect.Value{}); vals[0].String() != "Hello World" {
		t.Errorf("got %q, want %q", vals, "Hello World")
	}
	if !ran {
		t.Errorf("got %v, want %v", ran, true)
	}
	structure.Stub()
	if vals, _ := hello.Call(ctx, []reflect.Value{}); vals[0].String() != "" {
		t.Errorf("got %q, want %q", vals, "")
	}
}
