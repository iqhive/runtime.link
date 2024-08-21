package api

import (
	"errors"
	"reflect"

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
type Register[I any, V any] struct{}

func (Register[I, V]) addToStructure(field reflect.StructField, structure *Structure) {
	if structure.Instances == nil {
		structure.Instances = make(map[reflect.Type][]reflect.Type)
	}
	var itype = reflect.TypeOf([0]I{}).Elem()
	var value V
	structure.Instances[itype] = append(structure.Instances[itype], reflect.TypeOf(value))

	if itype == reflect.TypeOf([0]error{}).Elem() {
		variant, ok := any(value).(interface{ Reflection() []xyz.CaseReflection })
		if ok {
			var cases = variant.Reflection()
			for _, c := range cases {
				c := c
				var scenario Scenario
				scenario.Name = c.Name
				scenario.Kind = field.Name
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
		} else {

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
	Test func(error) bool
}

// Deprecated
var AccessDenied error = accessDenied{}

type accessDenied struct{}

func (accessDenied) Error() string {
	return "access denied"
}

func (accessDenied) StatusHTTP() int { return 403 }
