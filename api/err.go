package api

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"

	api_http "github.com/iqhive/runtime.link/api/internal/http"
	"github.com/iqhive/runtime.link/xyz"
)

var (
	ErrNotImplemented = api_http.ErrNotImplemented
	ErrAccessDenied   = accessDenied{}
)

// Error can be used to specify an enumerated set of error
// values that can be returned by an API endpoint. It behaves
// like a [xyz.Tagged] that implements [error].
type Error[T any] struct {
	errorMethods[T]
}

type errorMethods[T any] xyz.Tagged[error, T]

func (e errorMethods[T]) StatusHTTP() int {
	status, ok := e.Tag().Lookup("http")
	if !ok {
		return 500
	}
	code, err := strconv.Atoi(status)
	if err != nil {
		return 500
	}
	return code
}

func (e errorMethods[T]) Error() string {
	err, ok := e.Get()
	if !ok {
		return "empty error"
	}
	if err == nil {
		return e.String()
	}
	return err.Error()
}

func (e errorMethods[T]) apiError() {}

// errorResult returns err as a value of the function's error result, see
// [Function.Return] for how a type that err does not implement is handled.
func (fn Function) errorResult(err error) reflect.Value {
	rtype := fn.Type.Out(fn.Type.NumOut() - 1)
	if err == nil {
		return reflect.Zero(rtype)
	}
	target := reflect.New(rtype)
	if errors.As(err, target.Interface()) {
		return target.Elem()
	}
	for _, impl := range fn.registeredErrors(rtype) {
		if wrapped, ok := wrapError(impl, err); ok {
			return wrapped
		}
	}
	// Go cannot make up a value of an arbitrary interface, and the zero value
	// would make a failed call read as a success. reflect.MakeFunc panics here
	// anyway, so this only says why, and what to do about it.
	panic(fmt.Errorf("api: %s failed, but its declared error type %s cannot hold the error (%w): "+
		"register an implementation of %s with api.Register, with a case that takes an error "+
		"and has an http tag of 500 or above", fn.String(), rtype, err, rtype))
}

// registeredErrors returns the types registered for the API that can be
// held by rtype, including those registered for the bare [error] interface.
func (fn Function) registeredErrors(rtype reflect.Type) []reflect.Type {
	var registered []reflect.Type
	for _, itype := range []reflect.Type{rtype, errorType} {
		for _, impl := range fn.Root.Instances[itype] {
			if impl != nil && impl.AssignableTo(rtype) {
				registered = append(registered, impl)
			}
		}
	}
	return registered
}

// wrapError returns err inside the first case of impl that is able to report
// a failure on the server's side. It uses reflection because impl is only
// known at runtime, so [xyz.AccessorFor] cannot be used on it.
func wrapError(impl reflect.Type, err error) (reflect.Value, bool) {
	zero := reflect.Zero(impl)
	variant, ok := zero.Interface().(interface{ Reflection() []xyz.CaseReflection })
	if !ok {
		return reflect.Value{}, false
	}
	for i, c := range variant.Reflection() {
		if c.Vary != errorType || !isServerFailure(c.Tags) {
			continue
		}
		values := zero.MethodByName("Values")
		cases := values.Call([]reflect.Value{reflect.Zero(values.Type().In(0))})[0]
		if !cases.Type().Field(i).IsExported() {
			continue
		}
		return cases.Field(i).MethodByName("As").Call([]reflect.Value{reflect.ValueOf(&err).Elem()})[0], true
	}
	return reflect.Value{}, false
}

func isServerFailure(tags reflect.StructTag) bool {
	status, err := strconv.Atoi(tags.Get("http"))
	return err == nil && status >= 500
}

// failed reports whether an error result of a function holds a failure. An
// interface holds one unless it is nil, as err != nil decides in Go, so a nil
// pointer returned as an error is a failure. The zero value of a concrete error
// type means there is none, as no other value could say that a call succeeded.
func failed(result reflect.Value) bool {
	if result.Kind() == reflect.Interface {
		return !result.IsNil()
	}
	return !result.IsZero()
}

type registrator interface {
	addToStructure(reflect.StructField, *Structure)
}

// Register an implementation of an interface, if I is an [error] and
// V is an [Error]-type then each nested error value will be registered
// as a scenario, else V will be documented as a possible instance of
// I.
type Register[Interface any, Implementation any] struct{}

type Document[Interface any, Implementation any] = Register[Interface, Implementation]

func (Register[I, V]) addToStructure(field reflect.StructField, structure *Structure) {
	if structure.Instances == nil {
		structure.Instances = make(map[reflect.Type][]reflect.Type)
	}
	var itype = reflect.TypeOf([0]I{}).Elem()
	var value V
	structure.Instances[itype] = append(structure.Instances[itype], reflect.TypeOf(value))

	// If the interface being implemented is an error (either the bare [error]
	// interface registered at the API root, or a named per-endpoint error
	// interface such as ErrorForLogin) then expand each enumerated case of the
	// implementation into a [Scenario], stamped with the interface it belongs
	// to so that consumers can scope scenarios to the endpoints that declare
	// that interface as their error return.
	if itype.Implements(reflect.TypeOf([0]error{}).Elem()) {
		variant, ok := any(value).(interface{ Reflection() []xyz.CaseReflection })
		if ok {
			var cases = variant.Reflection()
			for _, c := range cases {
				c := c
				var scenario Scenario
				scenario.Name = c.Name
				scenario.Kind = field.Name
				scenario.Instance = itype
				scenario.Text = DocumentationOf(reflect.StructField{
					Name: c.Name,
					Tag:  c.Tags,
				})
				scenario.Tags = c.Tags
				scenario.Test = func(err error) bool {
					for e := err; e != nil; e = errors.Unwrap(e) {
						if c.Test(e) {
							return true
						}
					}
					return false
				}
				structure.Scenarios = append(structure.Scenarios, scenario)
			}
		}
	}
}

// Scenario documents an out-of-band signal supported by the API that
// requires actioning by the client, this could be an error, a
// redirection or a status.
type Scenario struct {
	Name string
	Kind string
	Text string
	Tags reflect.StructTag

	// Instance is the error interface this scenario was registered against
	// via [Register]. For API-wide registrations this is the bare [error]
	// interface; for per-endpoint error interfaces (such as ErrorForLogin)
	// it is that named interface, allowing scenarios to be scoped to the
	// endpoints that declare it as their error return type.
	Instance reflect.Type

	Test func(error) bool
}

// Deprecated
var AccessDenied error = accessDenied{}

type accessDenied struct{}

func (accessDenied) Error() string {
	return "access denied"
}

func (accessDenied) StatusHTTP() int { return 403 }
