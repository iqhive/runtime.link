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

	// Each case may be matched by JSON type, the first type
	// match that unmarshals without an error, will win.
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

The underlying memory representation can be designed to create switch values that
more efficiently make use of memory. Each value element within a case, will be packed
in to the available bytes of the switch type. Pointer-like-values are packed into an
available pointer field, of either a correctly typed pointer, a compatible interface,
or else an unsafe.Pointer. Fixed fields will be consumed first, followed by slices or
maps which will grow as needed. If the value cannot be stored in the available memory,
then a pointer will be used to store the value.

For example, the following switch type can store up to 8 bytes of data directly, with
an additional pointer field to catch larger values (such as strings).

	type container struct {
		raw [8]byte
		ptr unsafe.Pointer
	}
	type Any xyz.Switch[container, struct {
		B xyz.Case[Any, bool]
		I xyz.Case[Any, int]
		S xyz.Case[Any, string]
	}]
*/
package xyz

import (
	"encoding/json"
	"errors"
	"reflect"

	"runtime.link/api/xray"
)

type Validator interface {
	Validate(any) bool
}

type When[Variant isVariant, Constraint Validator] struct {
	_ [0]*Variant
	_ [0]*Constraint

	accessor *accessor
}

func (v When[Variant, Constraint]) String() string {
	return v.accessor.name
}

func (v When[Variant, Constraint]) value() Variant {
	var zero Variant
	return zero
}

// Maybe represents a value that is optional and can be omitted from the
// struct or function call it resides within. Not suitable for use as an
// underlying type.
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

// UnmarshalJSON implements the [json.Unmarshaler] interface.
func (o *Maybe[T]) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		clear(*o)
		return nil
	}
	var val T
	if err := json.Unmarshal(b, &val); err != nil {
		return xray.New(err)
	}
	clear(*o)
	if *o == nil {
		*o = New(val)
	} else {
		(*o)[ok{}] = val
	}
	return nil
}

// Static value that cannot be changed.
type Static[V isStatic[T], T any] struct {
	staticMethods[V, T]
}

func (s Static[V, T]) TypeJSON() reflect.Type { return reflect.TypeOf(s.Value()) }

type isStatic[T any] interface {
	Value() T
}

type staticMethods[V isStatic[T], T any] struct{}

func (v staticMethods[V, T]) Value() T {
	var zero V
	return zero.Value()
}

func (v staticMethods[V, T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Value())
}

func (v staticMethods[V, T]) UnmarshalJSON(data []byte) error {
	shouldBe, err := v.MarshalJSON()
	if err != nil {
		return err
	}
	if string(shouldBe) != string(data) {
		return errors.New("mismatched static value")
	}
	return nil
}
