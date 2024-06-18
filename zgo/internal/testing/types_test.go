package main

import "testing"

/*
A type determines a set of values together with operations and methods specific to those values.
A type may be denoted by a type name, if it has one, which must be followed by type arguments
if the type is generic. A type may also be specified using a type literal, which composes a
type from existing types.

The language predeclares certain type names. Others are introduced with type declarations or
type parameter lists. Composite types—array, struct, pointer, function, interface, slice, map,
and channel types—may be constructed using type literals.

Predeclared types, defined types, and type parameters are called named types. An alias denotes
a named type if the type given in the alias declaration is a named type.
*/
type Type struct{}

/*
A boolean type represents the set of Boolean truth values denoted by the predeclared constants
true and false. The predeclared boolean type is bool; it is a defined type.
*/
type Bool bool

/*
An integer, floating-point, or complex type represents the set of integer, floating-point, or
complex values, respectively. They are collectively called numeric types. The predeclared
architecture-independent numeric types are:
*/
type (
	Uint8      uint8
	Uint16     uint16
	Uint32     uint32
	Uint64     uint64
	Int8       int8
	Int16      int16
	Int32      int32
	Int64      int64
	Float32    float32
	Float64    float64
	Complex64  complex64
	Complex128 complex128
	Byte       byte
	Rune       rune
)

/*
The value of an n-bit integer is n bits wide and represented using two's complement arithmetic.

There is also a set of predeclared integer types with implementation-specific sizes:
*/
type (
	Uint    uint
	Int     int
	Uintptr uintptr
)

/*
To avoid portability issues all numeric types are defined types and thus distinct except byte,
which is an alias for uint8, and rune, which is an alias for int32. Explicit conversions are
required when different numeric types are mixed in an expression or assignment. For instance,
int32 and int are not the same type even though they may have the same size on a particular
architecture.
*/

/*
A string type represents the set of string values. A string value is a (possibly empty) sequence
of bytes. The number of bytes is called the length of the string and is never negative. Strings
are immutable: once created, it is impossible to change the contents of a string. The predeclared
string type is string; it is a defined type.

The length of a string s can be discovered using the built-in function len. The length is a
compile-time constant if the string is a constant. A string's bytes can be accessed by integer
indices 0 through len(s)-1. It is illegal to take the address of such an element; if s[i] is
the i'th byte of a string, &s[i] is invalid.
*/
type String string

/*
An array is a numbered sequence of elements of a single type, called the element type. The number
of elements is called the length of the array and is never negative.

The length is part of the array's type; it must evaluate to a non-negative constant representable
by a value of type int. The length of array a can be discovered using the built-in function len.
The elements can be addressed by integer indices 0 through len(a)-1. Array types are always
one-dimensional but may be composed to form multi-dimensional types.
*/
type (
	Array1 [32]byte
	Array2 [2 * N]struct{ x, y int32 }
	Array3 [1000]*float64
	Array4 [3][5]int
	Array5 [2][2][2]float64 // same as [2]([2]([2]float64))
)

/*
An array type T may not have an element of type T, or of a type containing T as a component, directly
or indirectly, if those containing types are only array or struct types.
*/
type (
	Array6 [10]*Array6              // T5 contains T5 as component of a pointer
	Array7 [10]func() Array7        // T6 contains T6 as component of a function type
	Array8 [10]struct{ f []Array8 } // T7 contains T7 as component of a slice in a struct
)

func TestTypeSlice(t *testing.T) {
	/*
		A slice is a descriptor for a contiguous segment of an underlying array and provides access to a
		numbered sequence of elements from that array. A slice type denotes the set of all slices of
		arrays of its element type. The number of elements is called the length of the slice and is never
		negative. The value of an uninitialized slice is nil.

		The length of a slice s can be discovered by the built-in function len; unlike with arrays it may
		change during execution. The elements can be addressed by integer indices 0 through len(s)-1. The
		slice index of a given element may be less than the index of the same element in the underlying array.

		A slice, once initialized, is always associated with an underlying array that holds its elements.
		A slice therefore shares storage with its array and with other slices of the same array; by
		contrast, distinct arrays always represent distinct storage.

		The array underlying a slice may extend past the end of the slice. The capacity is a measure of that
		extent: it is the sum of the length of the slice and the length of the array beyond the slice; a slice
		of length up to that capacity can be created by slicing a new one from the original slice. The capacity
		of a slice a can be discovered using the built-in function cap(a).

		A new, initialized slice value for a given element type T may be made using the built-in function make,
		which takes a slice type and parameters specifying the length and optionally the capacity. A slice
		created with make always allocates a new, hidden array to which the returned slice value refers.
		That is, executing

			make([]T, length, capacity)

		produces the same slice as allocating an array and slicing it, so these two expressions are equivalent:
	*/
	_ = make([]int, 50, 100)
	_ = new([100]int)[0:50]
	/*
		Like arrays, slices are always one-dimensional but may be composed to construct higher-dimensional
		objects. With arrays of arrays, the inner arrays are, by construction, always the same length; however
		with slices of slices (or arrays of slices), the inner lengths may vary dynamically. Moreover, the inner
		slices must be initialized individually.
	*/
}
