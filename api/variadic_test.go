package api_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/api"
)

type joinAPI struct {
	api.Specification
	Join func(ctx context.Context, sep string, parts ...string) (string, error)
}

func join(_ context.Context, sep string, parts ...string) (string, error) {
	return strings.Join(parts, sep), nil
}

func TestCopyWrapsVariadicFunction(t *testing.T) {
	a := joinAPI{Join: join}
	fn := api.StructureOf(&a).Functions[0]
	old := fn.Copy()
	fn.Make(func(ctx context.Context, args []reflect.Value) ([]reflect.Value, error) {
		return old.Call(ctx, args)
	})
	for want, parts := range map[string][]string{"a,b": {"a", "b"}, "": nil} {
		if got, err := a.Join(context.Background(), ",", parts...); got != want || err != nil {
			t.Errorf("Join(%q) = %q, %v", parts, got, err)
		}
	}
}

// Function.Call has always accepted variadic arguments one by one.
func TestCallSpreadsVariadicArguments(t *testing.T) {
	a := joinAPI{Join: join}
	fn := api.StructureOf(&a).Functions[0]
	args := func(values ...string) []reflect.Value {
		var args []reflect.Value
		for _, value := range values {
			args = append(args, reflect.ValueOf(value))
		}
		return args
	}
	ctx := context.Background()
	for want, values := range map[string][]string{"a-b": {"-", "a", "b"}, "c": {"-", "c"}, "": {"-"}} {
		got, err := fn.Call(ctx, args(values...))
		if err != nil || got[0].String() != want {
			t.Errorf("Call(%q) = %v, %v", values, got, err)
		}
	}
	got, err := fn.Call(ctx, append(args("-"), reflect.ValueOf([]string{"a", "b"})))
	if err != nil || got[0].String() != "a-b" {
		t.Errorf("Call with a slice = %v, %v", got, err)
	}
}

func TestCallWrapsVariadicWithoutErrorResult(t *testing.T) {
	var a struct {
		api.Specification
		Sum func(values ...int) int
	}
	a.Sum = func(values ...int) int { return len(values) }
	fn := api.StructureOf(&a).Functions[0]
	old := fn.Copy()
	fn.Make(func(ctx context.Context, args []reflect.Value) ([]reflect.Value, error) {
		return old.Call(ctx, args)
	})
	if got := a.Sum(1, 2, 3); got != 3 {
		t.Errorf("got %d", got)
	}
}

type variadicSuite struct {
	api.TestingFramework
	Svc joinAPI
}

func (s *variadicSuite) UseVariadic(ctx context.Context) error {
	got, err := s.Svc.Join(ctx, ",", "a", "b")
	if err != nil {
		return err
	}
	if got != "a,b" {
		return fmt.Errorf("got %q", got)
	}
	return nil
}

func TestDocumentationTracesVariadicFunction(t *testing.T) {
	var doc api.Documentation = func(context.Context) (api.Examples, error) {
		return &variadicSuite{Svc: joinAPI{Join: join}}, nil
	}
	example, ok := doc.Example(context.Background(), "UseVariadic")
	if !ok || example.Error != nil {
		t.Fatalf("ok=%v err=%v", ok, example.Error)
	}
	if len(example.Steps) != 1 {
		t.Fatalf("got %d steps", len(example.Steps))
	}
}
