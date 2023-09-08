// package link provides runtime.link reflection for structures and standard documented types.
package link

import (
	"fmt"
	"reflect"
	"strings"
)

// Tag should be embedded inside all runtime.link structures
// so that they can be documented. The struct tag for such
// fields will be considered to be a documentation comment.
// The first level of tab-indentation will be removed from
// each line of the doc string.
type Tag struct{}

// Structure is a runtime reflection representation of a runtime.link
// structure.
type Structure struct {
	Name string
	Docs string
	Tags reflect.StructTag

	Functions []Function
	Namespace map[string]Structure
}

// StructureOf returns a reflected runtime.link structure for
// the given value, if it is not a struct (or a pointer to a
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
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		value := rvalue.Field(i)
		tags, _, _ := strings.Cut(string(field.Tag), "\n")
		switch field.Type.Kind() {
		case reflect.Struct:
			if field.Type == reflect.TypeOf(Tag{}) {
				structure.Tags = reflect.StructTag(tags)
				structure.Docs = docs(field.Tag)
				continue
			}
			structure.Namespace[field.Name] = StructureOf(value)
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

// Stub each function in the structure.
func (s Structure) Stub() {
	for _, fn := range s.Functions {
		fn.Stub()
	}
	for _, child := range s.Namespace {
		child.Stub()
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

	Root Structure // root structure this function belongs to
	Path []string  // namespace path through root to reach this function.

	value reflect.Value
}

// Make the function use the given implementation, an error is returned
// if the implementation is not of the same type as the function.
func (fn Function) Make(impl any) error {
	rtype := reflect.TypeOf(impl)
	if rtype != fn.value.Type() {
		return fmt.Errorf("cannot implement %s of type %s with function of type  %s", fn.Name, fn.Type, rtype)
	}
	fn.value.Set(reflect.ValueOf(impl))
	return nil
}

// Stub the function with an empty implementation that returns zero values.
func (fn Function) Stub() {
	var results = make([]reflect.Value, fn.Type.NumOut())
	for i := range results {
		results[i] = reflect.Zero(fn.Type.Out(i))
	}
	fn.Make(reflect.MakeFunc(fn.Type, func(args []reflect.Value) []reflect.Value {
		return make([]reflect.Value, fn.Type.NumOut())
	}))
}

// Copy returns a copy of the function, it can be safely
// used inside of [Function.Make] in order to wrap the
// existing implementation.
func (fn Function) Copy() reflect.Value {
	val := reflect.New(fn.value.Type()).Elem()
	val.Set(fn.value)
	return val
}

// Call the function with the given arguments and return the
// results.
func (fn Function) Call(args []reflect.Value) []reflect.Value {
	return fn.value.Call(args)
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
