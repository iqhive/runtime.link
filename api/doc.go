package api

import (
	"context"
	"fmt"
	"iter"
	"reflect"
	"runtime/debug"
	"testing"
)

type literal string

type Documentation func(context.Context) (Examples, error)

func (fn Documentation) Example(ctx context.Context, name string) (Example, bool) {
	if fn == nil {
		return Example{}, false
	}
	isolated, err := fn(ctx)
	if err != nil {
		return Example{
			Error: err,
		}, true
	}
	method := reflect.ValueOf(isolated).MethodByName(name)
	if !method.IsValid() {
		return Example{}, false
	}
	var (
		rtype  = reflect.TypeOf(isolated)
		rvalue = reflect.ValueOf(isolated)
	)
	example := isolated.example()
	// setup API capture
	for i := 0; i < rtype.Elem().NumField(); i++ {
		if !rtype.Elem().Field(i).IsExported() {
			continue
		}
		example.trace(StructureOf(rvalue.Elem().Field(i).Addr().Interface()))
	}
	example.Title = name
	writer, ok := method.Interface().(func(context.Context) error)
	if !ok {
		return Example{}, false
	}
	func() {
		defer func() {
			if err := recover(); err != nil {
				example.Error = fmt.Errorf("panic %v %s", err, string(debug.Stack()))
				example.Panic = true
			}
		}()
		if err := writer(context.Background()); err != nil {
			example.Error = err
		}
	}()
	return *example, true
}

func (fn Documentation) Examples(ctx context.Context) (iter.Seq[string], error) {
	if fn == nil {
		return nil, nil
	}
	template, err := fn(ctx)
	if err != nil {
		return nil, err
	}
	return func(yield func(string) bool) {
		var rtype = reflect.TypeOf(template)
		var value = reflect.ValueOf(template)
		for i := 0; i < rtype.NumMethod(); i++ {
			method := rtype.Method(i)
			if _, ok := value.Method(i).Interface().(func(context.Context) error); !ok {
				continue
			}
			if !yield(method.Name) {
				break
			}
		}
	}, nil
}

type Example struct {
	Title string
	Tests string
	Story string
	Steps []Step
	Error error
	Panic bool

	depth uint
	setup bool
}

type Step struct {
	Note string
	Call *Function
	Args []reflect.Value
	Vals []reflect.Value

	Error error
	Depth uint
	Setup bool
}

type TestingFramework struct {
	eg Example
}

var _ WithExamples = (*Documentation)(nil)

type WithExamples interface {
	Example(context.Context, string) (Example, bool)
	Examples(context.Context) (iter.Seq[string], error)
}

type Examples interface {
	example() *Example
}

func (tdd *TestingFramework) example() *Example         { return &tdd.eg }
func (tdd *TestingFramework) Story(description literal) { tdd.eg.Story = string(description) }
func (tdd *TestingFramework) Tests(description literal) { tdd.eg.Tests = string(description) }
func (tdd *TestingFramework) Setup(ctx context.Context, fn func(ctx context.Context) error) error {
	tdd.eg.setup = true
	defer func() {
		tdd.eg.setup = false
	}()
	if err := fn(ctx); err != nil {
		return err
	}
	return nil
}

func (tdd *TestingFramework) Guide(description literal) {
	if len(tdd.eg.Steps) == 0 {
		tdd.eg.Steps = make([]Step, 1)
	}
	if tdd.eg.Steps[0].Note == "" {
		tdd.eg.Steps[0].Note = string(description)
	} else {
		tdd.eg.Steps = append(tdd.eg.Steps, Step{Note: string(description)})
	}
}

func (eg *Example) trace(spec Structure) {
	for i, old := range spec.Functions {
		old := old.Copy()
		fn := &spec.Functions[i]
		fn.Make(func(ctx context.Context, args []reflect.Value) (results []reflect.Value, err error) {
			eg.depth++
			defer func() {
				eg.depth--
			}()
			results, err = old.Call(ctx, args)
			if len(eg.Steps) == 0 {
				eg.Steps = append(eg.Steps, Step{})
			}
			step := &eg.Steps[len(eg.Steps)-1]
			if step.Call != nil {
				eg.Steps = append(eg.Steps, Step{})
				step = &eg.Steps[len(eg.Steps)-1]
			}
			step.Call = fn
			step.Args = args
			step.Vals = results
			step.Error = err
			step.Depth = eg.depth
			step.Setup = eg.setup
			return
		})
	}
	for _, section := range spec.Namespace {
		eg.trace(section)
	}
}

func Test(t *testing.T, impl Documentation) {
	examples, err := impl.Examples(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for name := range examples {
		example, _ := impl.Example(t.Context(), name)
		if example.Error != nil {
			t.Errorf("example %s failed %v", name, example.Error)
		}
	}
}
