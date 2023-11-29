package api_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"runtime.link/api"
	"runtime.link/xyz"
)

func TestStructure(t *testing.T) {
	var ctx = context.Background()
	var Example struct {
		_ api.Specification `
			This is an example runtime.link structure.`
		HelloWorld func() string `tag:"value"
			returns "Hello World"`
	}
	Example.HelloWorld = func() string {
		return "Hello World"
	}
	structure := api.StructureOf(&Example)
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
}

func TestEquals(t *testing.T) {
	var Example struct {
		_ api.Specification `
			This is an example runtime.link structure.`
		HelloWorld func() string `tag:"value"
			returns "Hello World"`
	}
	var structure = api.StructureOf(&Example)

	if !structure.Functions[0].Is(&Example.HelloWorld) {
		t.Fatal("got false, want true")
	}
}

/*
TestErrors is meant to demonstrate how possible
error values can be clearly defined.
*/
func TestErrors(t *testing.T) {
	type Error api.Error[struct {
		Internal     xyz.Case[Error, error] `http:"500"`
		AccessDenied Error                  `http:"403"`
	}]
	var API struct {
		api.Specification

		DoSomething func(context.Context) error
	}

	var Errors = xyz.AccessorFor(Error.Values)

	API.DoSomething = func(ctx context.Context) error {
		return Errors.Internal.As(errors.New(
			"failure",
		))
	}

	fmt.Println(API.DoSomething(context.Background()))
}
