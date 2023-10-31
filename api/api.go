// Package api defines the standard runtime reflection representation for a runtime.link API structure.
// The functions in this package are typically only used to implement runtime.link layers (ie. drivers)
// so that the layer can either host, or link functions specified within the structure.
package api

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	api_http "runtime.link/api/internal/http"
	"runtime.link/api/xray"
)

var (
	ErrNotImplemented = api_http.ErrNotImplemented
)

// Specification should be embedded in all runtime.link API structures.
type Specification struct{}

// Linker that can link a runtime.link API structure up to a 'Host'
// implementation using the specified 'Connection' configuration.
type Linker[Host any, Conn any] interface {
	Link(Structure, Host, Conn) error
}

// Exporter that can export a runtime.link API structure using the
// specified 'Options' configuration.
type Exporter[Host any, Options any] interface {
	Export(Structure, Options) (Host, error)
}

// Import the given runtime.link API structure using the given transport, host
// and transport-specific configuration. If an error is returned by the linker
// all functions will be stubbed with an error implementation that returns the
// error returned by the linker.
func Import[API, Host, Conn any](T Linker[Host, Conn], host Host, conn Conn) API {
	var (
		api       API
		structure = StructureOf(&api)
	)
	if err := T.Link(structure, host, conn); err != nil {
		structure.MakeError(err)
	}
	return api
}

// Export the given runtime.link API structure using the given exporter and
// configuration.
func Export[API, H, Options any](exporter Exporter[H, Options], impl API, options Options) (H, error) {
	return exporter.Export(StructureOf(impl), options)
}

// Auth returns an error if the given Conn is not
// allowed to access the given function. Used to implement
// authentication and authorisation for API calls.
type Auth[Conn any] interface {
	// AssertHeader is called before the request is processed it
	// should confirm the identify of the caller. The context
	// returned will be passed to the function being called.
	Authenticate(Conn, Function) (context.Context, error)

	// AssertAccess is called after arguments have been passed
	// and before the function is called. It should assert that
	// the identified caller is allowed to access the function.
	Authorize(Conn, Function, []reflect.Value) error

	// Redact is called on any errors raised by the function, it
	// can be used to log and/or report this error, or to redact
	// any sensitive information from the error before it is
	// returned to the caller.
	Redact(context.Context, error) error
}

// Host used to document host tags that identify the location
// of the link layer's target.
type Host interface {
	host()
}

// Structure is the runtime reflection representation for a runtime.link
// API structure. In Go source, these are represented using Go structs with
// at least one function field. These runtime.link API structures can be be
// nested in order to organise functions into sensible namespaces.
//
// For example:
//
//	type Example struct {
//		HelloWorld func() string `tag:"value"
//			returns "Hello World"`
//
//		Math struct {
//			Add func(a, b int) int `tag:"value"
//				returns a + b`
//		}
//	}
//
// Each function field can have struct tags that specify how a particular
// link layer should link to, or host the function. The tags can contain
// any number of newlines, each subsequent line after the first will be
// treated as documentation for the function (tabs are stripped from each
// line).
type Structure struct {
	Name string
	Docs string
	Tags reflect.StructTag

	Host reflect.StructTag // host tag determined by GOOS.

	Functions []Function
	Namespace map[string]Structure
}

// StructureOf returns a reflected runtime.link API structure
// for the given value, if it is not a struct (or a pointer to a
// struct), only the name will be available.
func StructureOf(val any) Structure {
	if already, ok := val.(Structure); ok {
		return already
	}
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
			if field.Type == reflect.TypeOf(Specification{}) {
				structure.Tags = reflect.StructTag(tags)
				structure.Docs = documentationOf(field.Tag)
				structure.Host = field.Tag
				continue
			}
			if field.Type.Implements(reflect.TypeOf([0]Host{}).Elem()) {
				structure.Host = field.Tag
				for structure.Host == "" && field.Anonymous {
					field = field.Type.Field(0)
					structure.Host = field.Tag
				}
			}
			if !field.IsExported() {
				value = reflect.NewAt(value.Type(), value.Addr().UnsafePointer()).Elem()
			}
			structure.Namespace[field.Name] = StructureOf(value.Addr().Interface())
		case reflect.Interface:
			if field.Type.Implements(reflect.TypeOf([0]Host{}).Elem()) {
				structure.Host = field.Tag
				structure.Docs = documentationOf(field.Tag)
				continue
			}
		case reflect.Func:
			structure.Functions = append(structure.Functions, Function{
				Name:  field.Name,
				Docs:  documentationOf(field.Tag),
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

// Is returns true if the given pointer is the same as the
// underlying function implementation.
func (fn Function) Is(ptr any) bool { return fn.value.Addr().Interface() == ptr }

// Make the function use the given implementation, an error is returned
// if the implementation is not of the same type as the function.
func (fn Function) Make(impl any) {
	if rvalue, ok := impl.(reflect.Value); ok {
		impl = rvalue.Interface()
	}

	switch function := impl.(type) {
	case func(context.Context, []reflect.Value) ([]reflect.Value, error):
		fn.value.Set(reflect.MakeFunc(fn.Type, func(args []reflect.Value) (results []reflect.Value) {
			ctx := context.Background()
			if len(args) > 0 && args[0].Type() == reflect.TypeOf([0]context.Context{}).Elem() {
				ctx = args[0].Interface().(context.Context)
				args = args[1:]
			}
			results, err := function(ctx, args)
			if results == nil {
				results = make([]reflect.Value, fn.NumOut())
				for i := range results {
					results[i] = reflect.Zero(fn.Type.Out(i))
				}
			}
			if err != nil {
				if fn.Type.NumOut() > 0 && fn.Type.Out(fn.Type.NumOut()-1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
					results = append(results, reflect.ValueOf(err))
				} else {
					panic(err)
				}
			} else {
				if fn.Type.NumOut() > 0 && fn.Type.Out(fn.Type.NumOut()-1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
					results = append(results, reflect.Zero(fn.Type.Out(fn.Type.NumOut()-1)))
				}
			}
			return results
		}))
		return
	case func([]reflect.Value) []reflect.Value:
		fn.value.Set(reflect.MakeFunc(fn.Type, function))
		return
	}

	rtype := reflect.TypeOf(impl)
	if rtype != fn.value.Type() {
		fn.MakeError(fmt.Errorf("function implemented with wrong type %s (should be %s)", rtype, fn.Type))
		return
	}
	fn.value.Set(reflect.ValueOf(impl))
}

// Copy returns a copy of the function, the copy can be safely
// used inside of [Function.Make] in order to wrap the
// existing implementation.
func (fn Function) Copy() Function {
	val := reflect.New(fn.value.Type()).Elem()
	val.Set(fn.value)
	fn.value = val
	return fn
}

// Call the function, automatically handling the presence of the first [context.Context]
// argument or the last [error] return value.
func (fn Function) Call(ctx context.Context, args []reflect.Value) ([]reflect.Value, error) {
	if fn.Type.NumIn() > 0 && fn.Type.In(0) == reflect.TypeOf([0]context.Context{}).Elem() {
		args = append([]reflect.Value{reflect.ValueOf(ctx)}, args...)
	}
	if fn.Type.NumOut() > 0 && fn.Type.Out(fn.Type.NumOut()-1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		results := fn.value.Call(args)
		if err := results[len(results)-1].Interface(); err != nil {
			return nil, xray.Error(err.(error))
		}
		return results[:len(results)-1], nil
	}
	return fn.value.Call(args), nil
}

func (fn Function) In(i int) reflect.Type {
	return fn.Type.In(i + fn.Type.NumIn() - fn.NumIn())
}

// NumIn returns the number of arguments to the function except for
// the first argument if it is a [context.Context].
func (fn Function) NumIn() int {
	if fn.Type.NumIn() > 0 && fn.Type.In(0) == reflect.TypeOf([0]context.Context{}).Elem() {
		return fn.Type.NumIn() - 1
	}
	return fn.Type.NumIn()
}

// NumOut returns the number of return values for the function
// excluding the [error] value.
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

// documentationOf returns the doc string associated with a [Tag].
// The doc string begins after the first newline of the
// tag and ignores any tab characters inside it.
func documentationOf(tag reflect.StructTag) string {
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

// NewArgumentScanner returns a new argument scanner where format
// parameters are referring to the given arguments.
func NewArgumentScanner(args []reflect.Value) ArgumentScanner {
	return ArgumentScanner{args, 0}
}

// Scan returns the argument specified by the given format string.
// The format string can be either %v, %[n]v or a FieldName.
func (scanner *ArgumentScanner) Scan(format string) (reflect.Value, error) {
	switch {
	case format == "":
		return reflect.Value{}, xray.Error(errors.New("ffi.ArgumentScanner: empty format"))
	case format == "%v":
	case strings.HasPrefix(format, "%[") && strings.HasSuffix(format, "]v"):
		var n int
		if _, err := fmt.Sscanf(format, "%%[%d]v", &n); err != nil {
			return reflect.Value{}, xray.Error(errors.New("ffi.ArgumentScanner: invalid format"))
		}
		if n < 1 {
			return reflect.Value{}, xray.Error(errors.New("ffi.ArgumentScanner: invalid format"))
		}
		if scanner.n+n > len(scanner.args) {
			return reflect.Value{}, xray.Error(errors.New("ffi.ArgumentScanner: invalid format"))
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
		return reflect.Value{}, xray.Error(errors.New("ffi.ArgumentScanner: no argument named " + format))
	}
	if scanner.n < 0 || scanner.n >= len(scanner.args) {
		return reflect.Value{}, xray.Error(errors.New("ffi.ArgumentScanner: invalid argument index"))
	}
	scanner.n++
	return scanner.args[scanner.n-1], nil
}
