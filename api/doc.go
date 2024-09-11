package api

import (
	"context"
	"fmt"
	"reflect"
	"runtime/debug"
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

func (fn Documentation) Examples(ctx context.Context) ([]string, error) {
	if fn == nil {
		return nil, nil
	}
	template, err := fn(ctx)
	if err != nil {
		return nil, err
	}
	var rtype = reflect.TypeOf(template)
	var value = reflect.ValueOf(template)
	var examples []string
	for i := 0; i < rtype.NumMethod(); i++ {
		method := rtype.Method(i)
		if _, ok := value.Method(i).Interface().(func(context.Context) error); !ok {
			continue
		}
		examples = append(examples, method.Name)
	}
	return examples, nil
}

type Example struct {
	Title string
	Tests string
	Story string
	Steps []Step
	Error error
	Panic bool

	depth uint
}

type Step struct {
	Note string
	Call *Function
	Args []reflect.Value
	Vals []reflect.Value

	Error error
	Depth uint
}

type TestingFramework struct {
	eg Example
}

type WithExamples interface {
	Example(context.Context, string) (Example, bool)
	Examples(context.Context) ([]string, error)
}

type Examples interface {
	example() *Example
}

func (tdd *TestingFramework) example() *Example         { return &tdd.eg }
func (tdd *TestingFramework) Story(description literal) { tdd.eg.Story = string(description) }
func (tdd *TestingFramework) Tests(description literal) { tdd.eg.Tests = string(description) }

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
			if len(eg.Steps) == 0 {
				eg.Steps = append(eg.Steps, Step{})
			}
			step := &eg.Steps[len(eg.Steps)-1]
			if step.Call != nil {
				eg.Steps = append(eg.Steps, Step{})
				step = &eg.Steps[len(eg.Steps)-1]
			}
			results, err = old.Call(ctx, args)
			step.Call = fn
			step.Args = args
			step.Vals = results
			step.Error = err
			step.Depth = eg.depth
			return
		})
	}
	for _, section := range spec.Namespace {
		eg.trace(section)
	}
}
