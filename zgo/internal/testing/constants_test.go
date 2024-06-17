package main

import "testing"

func TestConstants(t *testing.T) {
	/*
		There are boolean constants, rune constants, integer constants, floating-point constants,
		complex constants, and string constants. Rune, integer, floating-point, and complex
		constants are collectively called numeric constants.
	*/
	const (
		_ = true
		_ = 'a'
		_ = 1
		_ = 1.0
		_ = 1.0i
		_ = "a"
	)
	/*
		A constant value is represented by a rune, integer, floating-point, imaginary, or string
		literal, an identifier denoting a constant, a constant expression, a conversion with a
		result that is a constant, or the result value of some built-in functions such as min or
		max applied to constant arguments, unsafe.Sizeof applied to certain values, cap or len
		applied to some expressions, real and imag applied to a complex constant and complex
		applied to numeric constants. The boolean truth values are represented by the predeclared
		constants true and false. The predeclared identifier iota denotes an integer constant.

		In general, complex constants are a form of constant expression and are discussed in that
		section.

		Numeric constants represent exact values of arbitrary precision and do not overflow.
		Consequently, there are no constants denoting the IEEE-754 negative zero, infinity, and
		not-a-number values.

		Constants may be typed or untyped. Literal constants, true, false, iota, and certain
		constant expressions containing only untyped constant operands are untyped.

		A constant may be given a type explicitly by a constant declaration or conversion, or
		implicitly when used in a variable declaration or an assignment statement or as an
		operand in an expression. It is an error if the constant value cannot be represented
		as a value of the respective type. If the type is a type parameter, the constant is
		converted into a non-constant value of the type parameter.

		An untyped constant has a default type which is the type to which the constant is
		implicitly converted in contexts where a typed value is required, for instance, in
		a short variable declaration such as i := 0 where there is no explicit type. The
		default type of an untyped constant is bool, rune, int, float64, complex128, or
		string respectively, depending on whether it is a boolean, rune, integer, floating-point,
		complex, or string constant.

		Implementation restriction: Although numeric constants have arbitrary precision in
		the language, a compiler may implement them using an internal representation with
		limited precision. That said, every implementation must:

			- Represent integer constants with at least 256 bits.
			- Represent floating-point constants, including the parts of a complex constant,
			  with a mantissa of at least 256 bits and a signed binary exponent of at least 16 bits.
			- Give an error if unable to represent an integer constant precisely.
			- Give an error if unable to represent a floating-point or complex constant due to overflow.
			- Round to the nearest representable constant if unable to represent a floating-point or
			  complex constant due to limits on precision.

			These requirements apply both to literal constants and to the result of evaluating constant expressions.
	*/
}
