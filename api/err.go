package api

import (
	"errors"
	"reflect"
	"strconv"

	api_http "runtime.link/api/internal/http"
	"runtime.link/xyz"
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
