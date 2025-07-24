// Package stub provides a stub [api.Linker] that returns empty values and specific errors.
package stub

import (
	"io"
	"reflect"
	"strings"

	"runtime.link/api"
)

// Reason records the reason why a stub is being used.
type Reason string

// Testing is the reason that a stub is being used.
const (
	Testing Reason = "testing"
	TODO    Reason = "TODO"
)

// API linker, error will be returned from any functions in the API
// that include an error. Functions that do not include an error will
// panic, if the error is not nil.
var API api.Linker[Reason, error] = linker{}

type linker struct{}

func (ld linker) Link(structure api.Structure, reason Reason, err error) error {
	for _, fn := range structure.Functions {
		ld.stub(fn, err)
	}
	for _, child := range structure.Namespace {
		if err := ld.Link(child, reason, err); err != nil {
			return err
		}
	}
	return nil
}

func (ld linker) stub(fn api.Function, err error) {
	var results = make([]reflect.Value, fn.Type.NumOut())
	for i := range results {
		switch fn.Type.Out(i) {
		case reflect.TypeFor[io.ReadCloser]():
			results[i] = reflect.ValueOf(io.NopCloser(strings.NewReader("")))
		default:
			results[i] = reflect.Zero(fn.Type.Out(i))
		}
	}
	hasError := fn.Type.NumOut() > 0 && fn.Type.Out(fn.Type.NumOut()-1).Implements(reflect.TypeOf((*error)(nil)).Elem())
	if hasError && err != nil {
		results[fn.Type.NumOut()-1] = reflect.ValueOf(err)
	}
	fn.Make(reflect.MakeFunc(fn.Type, func(args []reflect.Value) []reflect.Value {
		if !hasError && err != nil {
			panic(err)
		}
		return results
	}).Interface())
}
