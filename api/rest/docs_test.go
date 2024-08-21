package rest

import (
	"reflect"
	"testing"
)

func TestNamespaceName(t *testing.T) {

	type Generic[T any] struct{}
	type Thing struct{}

	type Complex[A, B any] struct{}

	namespace, name := namespaceName(reflect.TypeOf(Generic[Thing]{}))
	if namespace != "rest" {
		t.Fatal("unexpected value")
	}
	if name != "Generic[rest.Thing]" {
		t.Fatal("unexpected value")
	}

	namespace, name = namespaceName(reflect.TypeOf(Complex[Thing, Thing]{}))
	if namespace != "rest" {
		t.Fatal("unexpected value")
	}
	if name != "Complex[rest.Thing, rest.Thing]" {
		t.Fatal("unexpected value")
	}
}
