package api

import (
	"context"
	"fmt"
	"reflect"
	"runtime/debug"
	"testing"
)

type literal string

type Documentation struct {
	Examples Examples
}

// NumExamples returns the number of examples in the documentation.
func (docs Documentation) NumExamples() int {
	return docs.Examples.NumExamples()
}

func (docs Documentation) documentation() Documentation { return docs }

// Example returns the example at index i.
func (docs Documentation) Example(i int) Example { return docs.Examples.Example(i) }

type WithDocumentation interface {
	documentation() Documentation

	NumExamples() int
	Example(int) Example
}

type Example struct {
	Title string
	Tests string
	Story string
	Steps []Step
	Error error
	Panic string
}

type Step struct {
	Note string
	Call *Function
	Args []reflect.Value
	Vals []reflect.Value

	Error error
	Depth uint
}

func (docs *Documentation) Story(description literal) { docs.Examples.Story(description) }
func (docs *Documentation) Tests(description literal) { docs.Examples.Tests(description) }
func (docs *Documentation) Guide(description literal) { docs.Examples.Guide(description) }

func ExamplesOf[T WithDocumentation](runtime func(context.Context) (*T, error)) (Examples, error) {
	return examples(nil, runtime)
}

func Test[T WithDocumentation](t *testing.T, runtime func(context.Context) (*T, error)) {
	t.Helper()
	_, err := examples(t, runtime)
	if err != nil {
		t.Error(err)
	}
}

type Examples struct {
	slice []Example
	index int
	stepn int
	depth uint
}

func (es *Examples) NumExamples() int      { return len(es.slice) }
func (es *Examples) Example(i int) Example { return es.slice[i] }
func (es *Examples) Story(desc literal) {
	if es.slice == nil {
		es.slice = make([]Example, 1)
	}
	es.slice[es.index].Story = string(desc)
}
func (es *Examples) Guide(desc literal) {
	if es.slice == nil {
		es.slice = make([]Example, 1)
	}
	example := &es.slice[es.index]
	if len(example.Steps) == 0 {
		example.Steps = make([]Step, 1)
	}
	step := &example.Steps[es.stepn]
	if step.Note == "" {
		step.Note = string(desc)
	} else {
		example.Steps = append(example.Steps, Step{Note: string(desc)})
		es.stepn++
	}
}
func (es *Examples) Tests(desc literal) {
	if es.slice == nil {
		es.slice = make([]Example, 1)
	}
	es.slice[es.index].Tests = string(desc)
}

func (es *Examples) trace(spec Structure) {
	for i, old := range spec.Functions {
		old := old.Copy()
		fn := &spec.Functions[i]
		fn.Make(func(ctx context.Context, args []reflect.Value) (results []reflect.Value, err error) {
			es.depth++
			defer func() {
				es.depth--
			}()

			if es.slice == nil {
				es.slice = make([]Example, 1)
			}

			solution := &es.slice[es.index]
			cursorStep := &solution.Steps
			solution.Steps = append(solution.Steps, Step{})
			es.stepn++

			results, err = old.Call(ctx, args)

			solution = &es.slice[es.index]
			step := &solution.Steps[len(*cursorStep)-1]

			step.Call = fn
			step.Args = args
			step.Vals = results
			step.Error = err
			step.Depth = es.depth

			return
		})
	}
	for _, section := range spec.Namespace {
		es.trace(section)
	}
}

func examples[T WithDocumentation](t *testing.T, runtime func(context.Context) (*T, error)) (Examples, error) {
	if t != nil {
		t.Helper()
	}
	ctx := context.Background()
	template, err := runtime(ctx)
	if err != nil {
		return Examples{}, err
	}
	docs := (*template).documentation()
	examples := &docs.Examples
	for i := 0; i < reflect.TypeOf(template).NumMethod(); i++ {
		isolated, err := runtime(ctx)
		if err != nil {
			return Examples{}, err
		}
		var (
			rtype  = reflect.TypeOf(isolated)
			rvalue = reflect.ValueOf(isolated)
		)
		rvalue.Elem().FieldByName("Examples").Set(reflect.ValueOf(*examples))
		// setup API capture
		for i := 0; i < rtype.Elem().NumField(); i++ {
			if !rtype.Elem().Field(i).IsExported() {
				continue
			}
			impl, ok := rvalue.Elem().Field(i).Addr().Interface().(WithSpecification)
			if ok {
				examples.trace(StructureOf(impl))
			}
		}
		method := rtype.Method(i)
		writer, ok := rvalue.Method(i).Interface().(func(context.Context) error)
		if !ok {
			continue
		}
		fn := func(t *testing.T) {
			if t != nil {
				t.Parallel()
				t.Helper()
			}
			defer func() {
				if t != nil {
					t.Helper()
				}
				if err := recover(); err != nil {
					examples.slice = append(examples.slice, Example{
						Title: string(method.Name),
						Panic: fmt.Sprintf("%v %s", err, string(debug.Stack())),
					})
				}
			}()
			if err := writer(context.Background()); err != nil {
				examples.slice = append(examples.slice, Example{
					Title: string(method.Name),
					Error: err,
				})
				if t != nil {
					t.Log(err)
					t.Fail()
				}
			}
		}
		if t != nil {
			t.Run(string(method.Name), fn)
		} else {
			fn(nil)
		}
		*examples = rvalue.Elem().FieldByName("Examples").Interface().(Examples)
	}
	return *examples, nil
}
