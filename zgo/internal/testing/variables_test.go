package main

import "testing"

func TestVariables(t *testing.T) {
	/*
		A variable is a storage location for holding a value. The set of permissible values
		is determined by the variable's type.

		A variable declaration or, for function parameters and results, the signature of a
		function declaration or function literal reserves storage for a named variable.
		Calling the built-in function new or taking the address of a composite literal
		allocates storage for a variable at run time. Such an anonymous variable is
		referred to via a (possibly implicit) pointer indirection.

		Structured variables of array, slice, and struct types have elements and fields
		that may be addressed individually. Each such element acts like a variable.

		The static type (or just type) of a variable is the type given in its declaration,
		the type provided in the new call or composite literal, or the type of an element
		of a structured variable. Variables of interface type also have a distinct dynamic
		type, which is the (non-interface) type of the value assigned to the variable at
		run time (unless the value is the predeclared identifier nil, which has no type).
		The dynamic type may vary during execution but values stored in interface variables
		are always assignable to the static type of the variable.
	*/
	type T struct{}
	var x interface{} // x is nil and has static type interface{}
	var v *T          // v has value nil, static type *T
	x = 42            // x has value 42 and dynamic type int
	x = v             // x has value (*T)(nil) and dynamic type *T
	_ = x

	/*
		A variable's value is retrieved by referring to the variable in an expression; it
		is the most recent value assigned to the variable. If a variable has not yet been
		assigned a value, its value is the zero value for its type.
	*/
}
