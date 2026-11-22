/*
Package xyz provides switch types, tuples and a binary sequence tag.

# Tuples

Tuples enable the repesentation of small sequences of values, where each value can have a
different type. Tuples are marshaled as JSON arrays and include a method for extracting
the values.

	pair := xyz.NewPair(1, 2)
	x, y := pair.Split()

	trio := xyz.NewTrio(1, "hello", "world")
	x, y, z := trio.Split()

	quad := xyz.NewQuad(1, 2.0, 3, "4")
	a, b, c, d := quad.Split()

Tuples can be useful in situations where a single value is expected but where
it would be more natural to use multiple values instead. For example, they can
be used to capture function arguments, or to pack multiple types into a
single generic type parameter.

# Switch Types (Enums)

Switch types can be used to switch on a set of known values.

To represent an enumerated type (enum) where each value must be distinct you can add
fields to the switch type with the same type as the switch itself.

	type Animal xyz.Switch[int, struct {
		Cat Animal
		Dog Animal
	}]

Note that neither switch types do not restrict the underlying value in memory to the set
of values defined in the switch type, so a default case should be included for any
switch statements on the value of a switch type.

# Tagged Types (Unions)

Tagged types are used to represent a discriminated set of values.

Each tagged case can have a variable value.

	type MyValue xyz.Tagged[any, struct {
		String xyz.Case[MyValue, string]
		Number xyz.Case[MyValue, float64]
	}]

If you use a custom interface type as the underlying type to switch on, you can define
a helper type to help ensure each case is asserted to implement that interface.

	type is[T io.Reader] xyz.Case[MyValue, T]

	type MyValue xyz.Switch[io.Reader, struct {
		Bufio is[*bufio.Reader]
		Bytes is[*bytes.Reader]
	}]

In order to create a new tagged value, or to assess the value of a switch, you must
create an accessor for the switch type. This is done by calling the Values method on the
switch type. Typically this should be performed once and stored in a variable, rather than
called on demand.

	// the convention is to use either a plural form, or to add a New or For prefix to
	// the type name or a Values suffix.
	var (
		NewAnimal    = xyz.AccessorFor(Animal.Values)
		Animals      = xyz.AccessorFor(Animal.Values)
		AnimalValues = xyz.AccessorFor(Animal.Values)
	)

The accessor provides methods for creating new values, and for assessing the class of
value.

	var hello = NewHelloWorld.Hello.As("hello")
	var value = MyValues.Number.As(22)
	var animal = Animals.Cat

	switch xyz.ValueOf(hello) {
	case NewHelloWorld.Hello:
		fmt.Println(NewHelloWorld.Raw())
	case NewHelloWorld.World:
		fmt.Println(NewHelloWorld.Raw())
	default:
	}

	// union values can be accesed using the Get method.
	switch xyz.ValueOf(value) {
	case MyValues.String:
		fmt.Println(MyValues.String.Get(value))
	case MyValues.Number:
		fmt.Println(MyValues.Number.Get(value))
	default:
	}

	// enum fields within a switch can be switched on directly.
	switch animal {
	case Animals.Cat:
	case Animals.Dog:
	default:
	}

Tagged values have builtin support for JSON marshaling and unmarshaling. The behaviour
of this can be controlled with json tags. [Enum]-backed values are always marshaled as
strings, switches with variable [Case] values need to be tagged in order to enable
the values to be unmarshaled.

	type Object struct {
		Field string `json:"field"`
	}

	// Each case may be matched by JSON type. The cases are tried in
	// the order they are declared, and the first that matches wins.
	// If its value fails to unmarshal, a later case is only used if
	// it unmarshals the JSON and marshals it back unchanged.
	type MyValue xyz.Switch[any, struct {
		Null  MyValue `json:",null"`
		String xyz.Case[MyValue, string]  `json:",string"`
		Number xyz.Case[MyValue, float64] `json:",number"`
		Object xyz.Case[MyValue, Object]  `json:",object"`
		Array  xyz.Case[MyValue, []int]   `json:",array"`
	}]

	MyValue.String.As("hello").MarshalJSON() // "hello"
	MyValue.Number.As(22).MarshalJSON() 	 // 22

	// An implicit object can be used with different field names
	// for each case.
	type MyValue xyz.Switch[any, struct {
		String xyz.Case[MyValue, string]  `json:"string"`
		Number xyz.Case[MyValue, float64] `json:"number"`
	}]

	MyValue.String.As("hello").MarshalJSON() // {"string": "hello"}
	MyValue.Number.As(22).MarshalJSON() 	 // {"number": 22}

	// A discrimator field can be specified.
	type MyValue xyz.Switch[any, struct {
		String xyz.Case[MyValue, string]  `json:"value?type=string"`
		Number xyz.Case[MyValue, float64] `json:"value?type=number"`
		Struct xyz.Case[MyValue, Object]  `json:"?type=struct"`
	}]

	MyValue.String.As("hello").MarshalJSON()        // {"value": "hello", "type": "string"}
	MyValue.Number.As(22).MarshalJSON()             // {"value": 22, "type": "number"}
	MyValue.Struct.As(Object{"1234"}).MarshalJSON() // {"type": "struct", "field": "1234"}

The storage type decides which values the cases of a tagged type can have. Every
case has to fit its storage, or the accessor panics when it is first asked for:

  - an interface type, such as any, holds a value of any case that implements it;
  - a string holds the text of a case, so every case needs a txt tag;
  - an unsafe.Pointer holds a pointer to a copy of the value of any case;
  - a slice of a type without pointers holds a value without pointers, of any size;
  - any other type without pointers, such as a number or an array of bytes, holds
    a value without pointers that is no larger than the storage is.

Nothing is packed, and no pointer is used for a value that does not fit. For example,
the following tagged type holds its values in 8 bytes, without an allocation, and
could not have a case for a string, which has a pointer.

	type Scalar xyz.Tagged[[8]byte, struct {
		B xyz.Case[Scalar, bool]
		I xyz.Case[Scalar, int64]
		F xyz.Case[Scalar, float64]
	}]
*/
package xyz

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/iqhive/runtime.link/api/xray"
)

type Validator interface {
	Validate(any) bool
}

// When is reserved for a case whose values are constrained by a [Validator]
// instead of by a type. It is not implemented: it has no way to make or read a
// value, and a [Tagged] type that declares one panics when it is first used,
// naming the field. Declare the case as a [Case].
type When[Variant isVariant, Constraint Validator] struct {
	_ [0]*Variant
	_ [0]*Constraint

	accessor *accessor
}

// String returns the name of the case, which nothing sets while When is not
// implemented.
func (v When[Variant, Constraint]) String() string {
	if v.accessor == nil {
		return ""
	}
	return v.accessor.name
}

// when marks the type for [notACase].
func (v When[Variant, Constraint]) when() {}

func (v When[Variant, Constraint]) value() Variant {
	var zero Variant
	return zero
}

// Maybe represents a value that is optional and can be omitted from the
// struct or function call it resides within. Not suitable for use as an
// underlying type. Nothing changes a Maybe once it is made, so a copy of one
// can be decoded into without changing the original.
type Maybe[T any] map[ok]T

func (o Maybe[T]) TypeJSON() reflect.Type { return reflect.TypeOf([0]T{}).Elem() }

type ok struct{}

// New returns an un-omitted (included) [Maybe] value.
// Calls to [Get] will return this value and true.
func New[T any](val T) Maybe[T] {
	var omit = make(map[ok]T)
	omit[ok{}] = val
	return omit
}

// Get returns the value and true if the value was included
// otherwise it returns the zero value and false.
func (o Maybe[T]) Get() (T, bool) {
	val, ok := o[ok{}]
	return val, ok
}

// MarshalJSON implements the [json.Marshaler] interface.
func (o Maybe[T]) MarshalJSON() ([]byte, error) {
	if val, ok := o.Get(); ok {
		return json.Marshal(val)
	}
	return []byte("null"), nil
}

// UnmarshalJSON implements the [json.Unmarshaler] interface. It replaces the
// map instead of writing to it, because a copy of a Maybe shares its map: the
// value that a struct was copied from would change with the copy. null leaves
// a nil Maybe nil and any other empty.
func (o *Maybe[T]) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		if *o != nil {
			*o = Maybe[T]{}
		}
		return nil
	}
	var val T
	if err := json.Unmarshal(b, &val); err != nil {
		return xray.New(err)
	}
	*o = New(val)
	return nil
}

// Static value that cannot be changed.
type Static[V isStatic[T], T any] struct {
	staticMethods[V, T]
}

type isStatic[T any] interface {
	Value() T
}

type staticMethods[V isStatic[T], T any] struct{}

func (v staticMethods[V, T]) Value() T {
	var zero V
	return zero.Value()
}

func (v staticMethods[V, T]) TypeJSON() reflect.Type { return reflect.TypeOf(v.Value()) }

func (v staticMethods[V, T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Value())
}

func (v staticMethods[V, T]) UnmarshalJSON(data []byte) error {
	shouldBe, err := v.MarshalJSON()
	if err != nil {
		return err
	}
	want, err := decodeExactly(shouldBe)
	if err != nil {
		return err
	}
	got, err := decodeExactly(data)
	if err != nil || !reflect.DeepEqual(got, want) {
		return errors.New("mismatched static value")
	}
	return nil
}

// decodeExactly decodes JSON to compare it with other JSON, as one value can be
// written in many ways. A float64 would lose the difference between large numbers.
func decodeExactly(data []byte) (any, error) {
	if !json.Valid(data) {
		return nil, errors.New("invalid JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	err := decoder.Decode(&value)
	return value, err
}
