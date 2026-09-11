package api_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/xyz"
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
	if hello.Args != nil {
		t.Errorf("Args = %q, want nil for non-opted-in structure", hello.Args)
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

// Nested sibling namespaces used to share append's backing array, so later
// children could overwrite earlier Function.Path values.
func TestFunctionPathNoAliasing(t *testing.T) {
	var API struct {
		api.Specification
		One struct {
			Two struct {
				Three struct {
					Alpha struct {
						A func()
					}
					Beta struct {
						B func()
					}
				}
			}
		}
	}
	got := map[string][]string{}
	for fn := range api.StructureOf(&API).Iter() {
		got[fn.Name] = fn.Path
	}
	want := map[string][]string{
		"A": {"One", "Two", "Three", "Alpha"},
		"B": {"One", "Two", "Three", "Beta"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Path = %v, want %v", got, want)
	}
	for name, path := range got {
		if cap(path) != len(path) {
			t.Errorf("%s Path len=%d cap=%d, want exact capacity", name, len(path), cap(path))
		}
	}
}

type printCycleAPI struct {
	api.Specification
	Hello func()
	Math  struct {
		Add func(a, b int) int
	}
}

func TestString(t *testing.T) {
	structure := api.StructureOf(&printCycleAPI{})
	if got, want := structure.String(), "printCycleAPI"; got != want {
		t.Errorf("Structure.String() = %q, want %q", got, want)
	}
	hello := structure.Functions[0]
	if got, want := hello.String(), "Hello"; got != want {
		t.Errorf("Function.String() = %q, want %q", got, want)
	}
	add := structure.Namespace["Math"].Functions[0]
	if got, want := add.String(), "Math.Add"; got != want {
		t.Errorf("nested Function.String() = %q, want %q", got, want)
	}

	// Printing used to overflow through Function.Root ↔ Structure.Functions.
	for _, v := range []any{structure, hello, add, api.Structure{}, api.Function{}} {
		_ = fmt.Sprintf("%v", v)
		_ = fmt.Sprintf("%s", v)
		_ = fmt.Sprintf("%+v", v)
		_ = fmt.Sprintf("%#v", v)
		_ = fmt.Sprint(v)
	}
	if got, want := fmt.Sprintf("%q", add), `"Math.Add"`; got != want {
		t.Errorf("%%q = %s, want %s", got, want)
	}
	if got, want := fmt.Sprintf("%#v", add), "Math.Add"; got != want {
		t.Errorf("%%#v = %q, want %q", got, want)
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
		Internal xyz.Case[Error, error] `http:"500"
			internal server error`
		AccessDenied Error `http:"403"
			access denied`
	}]
	var Errors = xyz.AccessorFor(Error.Values)

	type Redirect api.Error[struct {
		Standard xyz.Case[Error, error] `http:"302"`
	}]
	var Redirects = xyz.AccessorFor(Redirect.Values)
	_ = Redirects

	var API struct {
		api.Specification

		Error    api.Register[error, Error]
		Redirect api.Register[error, Redirect]

		DoSomething func(context.Context) error
	}

	API.DoSomething = func(ctx context.Context) error {
		return Errors.Internal.As(errors.New(
			"failure",
		))
	}

	if fmt.Sprint(API.DoSomething(context.Background())) != "failure" {
		t.Error("expected failure")
	}

	var structure = api.StructureOf(&API)
	if len(structure.Scenarios) != 3 {
		t.Errorf("got %d scenarios, want %d", len(structure.Scenarios), 3)
	}
	if structure.Scenarios[0].Name != "Internal" {
		t.Errorf("got %q, want %q", structure.Scenarios[0].Name, "Internal")
	}
}

type readyExamples struct {
	api.TestingFramework
}

// FailReady is marked ready for regression testing and fails.
func (eg *readyExamples) FailReady(ctx context.Context) error {
	eg.Ready()
	return errors.New("boom")
}

// FailNotReady is not marked ready and fails; it should be skipped rather than
// failing the build.
func (eg *readyExamples) FailNotReady(ctx context.Context) error {
	return errors.New("boom")
}

// PassNotReady is not marked ready and passes.
func (eg *readyExamples) PassNotReady(ctx context.Context) error {
	return nil
}

func newReadyExamples(ctx context.Context) (api.Examples, error) {
	return &readyExamples{}, nil
}

// TestReadyGate verifies that only examples which call Ready() are examined for
// the regression-failure flag, and that the flag is not leaked between the
// documentation render and the recorded example.
func TestReadyGate(t *testing.T) {
	var doc api.Documentation = newReadyExamples

	ready, ok := doc.Example(context.Background(), "FailReady")
	if !ok {
		t.Fatal("FailReady not found")
	}
	if !ready.Ready {
		t.Error("FailReady should be marked Ready")
	}
	if ready.Error == nil {
		t.Error("FailReady should have recorded its error")
	}

	notReady, ok := doc.Example(context.Background(), "FailNotReady")
	if !ok {
		t.Fatal("FailNotReady not found")
	}
	if notReady.Ready {
		t.Error("FailNotReady should not be marked Ready")
	}
	if notReady.Error == nil {
		t.Error("FailNotReady should still record its error for reporting")
	}
}
