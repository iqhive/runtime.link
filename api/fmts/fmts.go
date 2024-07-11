package fmts

import (
	"context"
	"fmt"
	"reflect"

	"runtime.link/api"
)

var API api.Linker[string, func(string, ...any) string] = linker{}

type linker struct{}

func (l linker) Link(structure api.Structure, host string, sprintf func(string, ...any) string) error {
	for _, fn := range structure.Functions {
		if fn.NumOut() != 1 || fn.Type.Out(0).Kind() != reflect.String {
			return fmt.Errorf("function %s must have exactly one string return value", fn.Name)
		}
		stype := fn.Type.Out(0)
		fn.Make(func(ctx context.Context, args []any) ([]any, error) {
			var svalue = reflect.New(stype).Elem()
			svalue.SetString(sprintf(fn.Tags.Get("fmts"), args...))
			return []any{svalue.Interface()}, nil
		})
	}
	for _, sub := range structure.Namespace {
		l.Link(sub, host, sprintf)
	}
	return nil
}
