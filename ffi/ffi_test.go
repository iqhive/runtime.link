package ffi_test

import (
	"reflect"
	"testing"

	"runtime.link/ffi"
)

func TestStructure(t *testing.T) {
	var Example struct {
		_ ffi.Documentation `
			This is an example runtime.link structure.`
		HelloWorld func() string `tag:"value"
			returns "Hello World"`
	}
	Example.HelloWorld = func() string {
		return "Hello World"
	}
	structure := ffi.StructureOf(&Example)
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
	if val := hello.Copy().Call([]reflect.Value{})[0].String(); val != "Hello World" {
		t.Errorf("got %q, want %q", val, "Hello World")
	}
	var ran bool
	var old = hello.Copy()
	var wrap = func() string {
		ran = true
		return old.Call([]reflect.Value{})[0].String()
	}
	hello.Make(wrap)
	if val := hello.Copy().Call([]reflect.Value{})[0].String(); val != "Hello World" {
		t.Errorf("got %q, want %q", val, "Hello World")
	}
	if !ran {
		t.Errorf("got %v, want %v", ran, true)
	}
	structure.Stub()
	if val := hello.Copy().Call([]reflect.Value{})[0].String(); val != "" {
		t.Errorf("got %q, want %q", val, "")
	}
}
