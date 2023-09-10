// package ffi provides reflection for runtime.link structures.
package ffi

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// Documentation should be embedded inside all runtime.link FFI
// structures and types  so that they can be documented. The
// struct tag for such fields will be considered to be a comment.
// The first level of tab-indentation will be removed from
// each subsequent line of the struct tag.
type Documentation struct{}

// Host used to document host tags that identify the location
// of the link layer's target.
type Host interface {
	host()
}

// Structure is a runtime reflection representation for a runtime.link
// structure.
type Structure struct {
	Name string
	Docs string
	Tags reflect.StructTag

	Host reflect.StructTag // host tag determined by GOOS.

	Functions []Function
	Namespace map[string]Structure
}

// StructureOf returns a reflected runtime.link structure
// for the given value, if it is not a struct (or a pointer to a
// struct), only the name will be available.
func StructureOf(val any) Structure {
	rtype := reflect.TypeOf(val)
	rvalue := reflect.ValueOf(val)
	for rtype.Kind() == reflect.Ptr {
		rtype = rtype.Elem()
		if !rvalue.IsNil() {
			rvalue = rvalue.Elem()
		}
	}
	var structure Structure
	structure.Name = rtype.Name()
	structure.Namespace = make(map[string]Structure)
	if rtype.Kind() != reflect.Struct {
		return structure
	}
	if !rvalue.CanAddr() {
		copy := reflect.New(rtype).Elem()
		copy.Set(rvalue)
		rvalue = copy
	}
	goos, ok := rtype.FieldByName(runtime.GOOS)
	if ok {
		structure.Host = goos.Tag
	}
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		value := rvalue.Field(i)
		tags, _, _ := strings.Cut(string(field.Tag), "\n")
		switch field.Type.Kind() {
		case reflect.Struct:
			if field.Type == reflect.TypeOf(Documentation{}) {
				structure.Tags = reflect.StructTag(tags)
				structure.Docs = docs(field.Tag)
				continue
			}
			if field.Type.Implements(reflect.TypeOf([0]Host{}).Elem()) {
				structure.Host = field.Tag
				for structure.Host == "" && field.Anonymous {
					field = field.Type.Field(0)
					structure.Host = field.Tag
				}
			}
			structure.Namespace[field.Name] = StructureOf(value.Addr().Interface())
		case reflect.Interface:
			if field.Type.Implements(reflect.TypeOf([0]Host{}).Elem()) {
				structure.Host = field.Tag
				continue
			}
		case reflect.Func:
			structure.Functions = append(structure.Functions, Function{
				Name:  field.Name,
				Docs:  docs(field.Tag),
				Tags:  reflect.StructTag(tags),
				Type:  field.Type,
				value: value,
			})
		}
	}
	for _, fn := range structure.Functions {
		fn.Root = structure
	}
	for name, child := range structure.Namespace {
		child.Name = name
		child.link([]string{name})
		structure.Namespace[name] = child
	}
	return structure
}

// Stub calls [Function.Stub] on each function within
// the structure.
func (s Structure) Stub() {
	for _, fn := range s.Functions {
		fn.Stub()
	}
	for _, child := range s.Namespace {
		child.Stub()
	}
}

// MakeError calls [Function.MakeError] on each function
// within the structure.
func (s Structure) MakeError(err error) {
	for _, fn := range s.Functions {
		fn.MakeError(err)
	}
	for _, child := range s.Namespace {
		child.MakeError(err)
	}
}

func (s *Structure) link(path []string) {
	for i := range s.Functions {
		s.Functions[i].Path = path
	}
	for name, child := range s.Namespace {
		child.link(append(path, name))
		s.Namespace[name] = child
	}
}

// Function is a runtime reflection representation of a runtime.link
// function.
type Function struct {
	Name string
	Docs string
	Tags reflect.StructTag
	Type reflect.Type

	Root Structure // root structure this function belongs to.
	Path []string  // namespace path from root to reach this function.

	value reflect.Value
}

// Make the function use the given implementation, an error is returned
// if the implementation is not of the same type as the function.
func (fn Function) Make(impl any) {
	if rvalue, ok := impl.(reflect.Value); ok {
		impl = rvalue.Interface()
	}
	rtype := reflect.TypeOf(impl)
	if rtype != fn.value.Type() {
		fn.MakeError(fmt.Errorf("function implemented with wrong type %s (should be %s)", fn.Type, rtype))
		return
	}
	fn.value.Set(reflect.ValueOf(impl))
}

// Copy returns a copy of the function, it can be safely
// used inside of [Function.Make] in order to wrap the
// existing implementation.
func (fn Function) Copy() reflect.Value {
	val := reflect.New(fn.value.Type()).Elem()
	val.Set(fn.value)
	return val
}

// Stub the function with an empty implementation that returns zero values.
func (fn Function) Stub() {
	var results = make([]reflect.Value, fn.Type.NumOut())
	for i := range results {
		results[i] = reflect.Zero(fn.Type.Out(i))
	}
	fn.Make(reflect.MakeFunc(fn.Type, func(args []reflect.Value) []reflect.Value {
		return results
	}).Interface())
}

// NumOut returns the number of return values for the function
// excluding the error value.
func (fn Function) NumOut() int {
	out := fn.Type.NumOut()
	if out > 0 && fn.Type.Out(fn.Type.NumOut()-1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return out - 1
	}
	return out
}

// MakeError makes the function use the given error as its
// implementation. Either returning it (if possible) otherwise
// panicking with it.
func (fn Function) MakeError(err error) {
	out := fn.Type.NumOut()
	if out > 0 && fn.Type.Out(out-1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		var results = make([]reflect.Value, fn.Type.NumOut())
		for i := range results {
			results[i] = reflect.Zero(fn.Type.Out(i))
		}
		results[out-1] = reflect.ValueOf(err)
		fn.Make(reflect.MakeFunc(fn.Type, func(args []reflect.Value) []reflect.Value {
			return results
		}).Interface())
		return
	}
	fn.Make(reflect.MakeFunc(fn.Type, func(args []reflect.Value) []reflect.Value {
		panic(err)
	}))
}

// docs returns the doc string associated with a [Tag].
// The doc string begins after the first newline of the
// tag and ignores any tab characters inside it.
func docs(tag reflect.StructTag) string {
	splits := strings.SplitN(string(tag), "\n", 2)
	if len(splits) > 1 {
		var indentation int // determine the indentation on the first line
		for _, char := range splits[1] {
			if char != '\t' {
				break
			}
			indentation++
		}
		var sequence = strings.Repeat("\t", indentation)
		return strings.ReplaceAll("\n"+splits[1], "\n"+sequence, "\n")[1:]
	}
	return ""
}

// Stub returns a stubbed runtime.link structure such that each
// function returns zero values. Can be useful for mocking and
// tests.
func Stub[Structure any]() Structure {
	var value Structure
	StructureOf(&value).Stub()
	return value
}

// Return returns the given results, if err is not nil, then results can be
// nil and vice versa.
func (fn Function) Return(results []reflect.Value, err error) []reflect.Value {
	if results == nil {
		results = make([]reflect.Value, fn.Type.NumOut())
		for i := range results {
			results[i] = reflect.Zero(fn.Type.Out(i))
		}
	}
	for len(results) < fn.Type.NumOut() {
		results = append(results, reflect.Zero(fn.Type.Out(fn.Type.NumOut()-1)))
	}
	if err != nil {
		if fn.Type.NumOut() > 0 && fn.Type.Out(fn.Type.NumOut()-1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			results[fn.Type.NumOut()-1] = reflect.ValueOf(err)
			return results
		}
		panic(err)
	}
	return results
}

// ArgumentScanner can scan arguments via a formatting pattern.
// Either %v, %[n]v or FieldName
type ArgumentScanner struct {
	args []reflect.Value
	n    int
}

func NewArgumentScanner(args []reflect.Value) ArgumentScanner {
	return ArgumentScanner{args, 0}
}

func (scanner *ArgumentScanner) Scan(format string) (reflect.Value, error) {
	switch {
	case format == "":
		return reflect.Value{}, errors.New("ffi.ArgumentScanner: empty format")
	case format == "%v":
	case strings.HasPrefix(format, "%[") && strings.HasSuffix(format, "]v"):
		var n int
		if _, err := fmt.Sscanf(format, "%%[%d]v", &n); err != nil {
			return reflect.Value{}, errors.New("ffi.ArgumentScanner: invalid format")
		}
		if n < 1 {
			return reflect.Value{}, errors.New("ffi.ArgumentScanner: invalid format")
		}
		if scanner.n+n > len(scanner.args) {
			return reflect.Value{}, errors.New("ffi.ArgumentScanner: invalid format")
		}
		return scanner.args[scanner.n+n-1], nil
	default:
		for _, arg := range scanner.args {
			if arg.Kind() == reflect.Struct {
				rtype := arg.Type()
				for j := 0; j < rtype.NumField(); j++ {
					if rtype.Field(j).Name == format {
						return arg.Field(j), nil
					}
				}
			}
		}
		return reflect.Value{}, errors.New("ffi.ArgumentScanner: no argument named " + format)
	}
	if scanner.n < 0 || scanner.n >= len(scanner.args) {
		return reflect.Value{}, errors.New("ffi.ArgumentScanner: invalid argument index")
	}
	scanner.n++
	return scanner.args[scanner.n-1], nil
}
