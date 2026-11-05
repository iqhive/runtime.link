// Package fmts provides a format specification API linker. It can be used to represent how values are formatted and parsed.
package fmts

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/xray"
)

var API api.Linker[func(string, ...any) string, func(string, string, ...any) (int, error)] = linker{}

type linker struct{}

type candidate struct {
	format string
	prefix string
	suffix string
	search map[string]int
	author reflect.Value
}

// rank scores how much of value the format accounts for. A format that does
// not match the value scores 0, the same as a catch-all "%v", so the two tie
// and neither is chosen, while a lone format is chosen for any value and its
// scanner reports a value that does not fit.
func (c candidate) rank(value string) (score int) {
	if c.prefix != "" {
		if !strings.HasPrefix(value, c.prefix) {
			return 0
		}
		score++
	}
	if c.suffix != "" {
		if !strings.HasSuffix(value, c.suffix) {
			return 0
		}
		score++
	}
	for key, count := range c.search {
		if strings.Count(value, key) != count {
			return 0
		}
		score++
	}
	return score
}

var anyType = reflect.TypeOf([0]any{}).Elem()
var errType = reflect.TypeOf((*error)(nil)).Elem()

func (l linker) fill(structure api.Structure, sscanf func(string, string, ...any) (int, error), formats map[reflect.Type][]candidate) {
	for _, fn := range structure.Functions {
		if fn.NumOut() == 2 && fn.Type.Out(0) == anyType && fn.Type.Out(1) == errType {
			continue
		}

		// parsers
		if fn.NumIn() == 1 && fn.Type.In(0).Kind() == reflect.String && fn.Type.NumOut() > 1 && fn.Type.Out(fn.Type.NumOut()-1) == errType {
			fn.Make(func(ctx context.Context, args []reflect.Value) ([]reflect.Value, error) {
				var results = make([]any, fn.NumOut())
				for i := 0; i < fn.NumOut(); i++ {
					results[i] = reflect.New(fn.Type.Out(i)).Interface()
				}
				_, err := sscanf(args[0].String(), fn.Tags.Get("fmts"), results...)
				if err != nil {
					return nil, xray.New(err)
				}
				rvalues := make([]reflect.Value, len(results))
				for i := 0; i < len(results); i++ {
					rvalues[i] = reflect.ValueOf(results[i]).Elem()
				}
				return rvalues, nil
			})
			stype := fn.Type.In(0)
			tag, ok := fn.Tags.Lookup("fmts")
			if !ok {
				continue
			}
			prefix, format, ok := strings.Cut(tag, "%v")
			if !ok {
				format = prefix
				prefix = ""
			}
			split := strings.Split(format, "%v")
			var suffix string
			if !strings.HasSuffix(format, "%v") {
				suffix = split[len(split)-1]
			}
			var search = make(map[string]int)
			for i := 0; i < len(split)-1; i++ {
				search[split[i]] += 1
			}
			formats[stype] = append(formats[stype], candidate{
				format: tag,
				prefix: prefix,
				suffix: suffix,
				search: search,
				author: fn.Impl,
			})
		}
	}
	for _, sub := range structure.Namespace {
		l.fill(sub, sscanf, formats)
	}
}

func (l linker) Link(structure api.Structure, sprintf func(string, ...any) string, sscanf func(string, string, ...any) (int, error)) error {
	formats := make(map[reflect.Type][]candidate)
	if sscanf != nil {
		l.fill(structure, sscanf, formats)
	}
	return l.link(structure, sprintf, formats)
}

func (l linker) link(structure api.Structure, sprintf func(string, ...any) string, formats map[reflect.Type][]candidate) error {
	for _, fn := range structure.Functions {
		if fn.Type.NumOut() > 1 && fn.Type.Out(fn.Type.NumOut()-1) == errType {
			continue
		}
		// Parser Function.
		if fn.Type.NumOut() == 1 && fn.Type.Out(0) == anyType && fn.Type.NumIn() == 1 && fn.Type.In(0).Kind() == reflect.String {
			candidates := formats[fn.Type.In(0)]
			fn.Make(func(ctx context.Context, args []reflect.Value) ([]reflect.Value, error) {
				var value = args[0].String()
				var highest int = -1
				var matches int
				var sharing int
				for i, candidate := range candidates {
					score := candidate.rank(value)
					switch {
					case score == highest:
						sharing++
					case score > highest:
						highest = score
						matches = i
						sharing = 0
					}
				}
				if highest == -1 {
					return nil, nil
				}
				if sharing > 0 {
					return nil, nil
				}
				impl := candidates[matches].author
				results := make([]reflect.Type, impl.Type().NumOut())
				for i := 0; i < impl.Type().NumOut(); i++ {
					results[i] = impl.Type().Out(i)
				}
				wrapper := reflect.MakeFunc(reflect.FuncOf(nil, results, false), func([]reflect.Value) []reflect.Value {
					return impl.Call([]reflect.Value{args[0]})
				})
				author := reflect.New(anyType).Elem()
				author.Set(wrapper)
				return []reflect.Value{author}, nil
			})
			continue
		}
		// Format Function.
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
	// Parsers in a namespace choose from every format, as those at the top do.
	// A function in a namespace that cannot be linked is left nil rather than
	// failing the link, so that an API which never calls it links as before.
	for _, sub := range structure.Namespace {
		_ = l.link(sub, sprintf, formats)
	}
	return nil
}
