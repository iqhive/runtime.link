/*
Package xyz provides switch types, tuples and a standard binary sequence tag.

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

# Switch Types

Switch types are used to represent a discriminated set of values.

To represent an enumerated type (enum) where each value must be distinct you can add fields to
the switch type with the same type as the switch itself.

	type Animal xyz.Switch[xyz.Iota, struct {
		Cat Animal
		Dog Animal
	}]

Union types can also be represented, where each switch case can have a variable value.

	type MyValue xyz.Switch[any, struct {
		String xyz.Case[StringOrInt, string]
		Number xyz.Case[StringOrInt, float64]
	}]

In order to create a new switch value, or to assess the value of a switch, you must create
an accessor for the switch type. This is done by calling the Values method on the switch type.
Typically this should be performed once and stored in a variable, rather than called on demand.

	// the convention is to use either a plural form, or to add a New prefix to the type name.
	var (
		NewAnimal = xyz.AccessorFor(Animal.Values)
		Animals   = xyz.AccessorFor(Animal.Values)
	)

The accessor provides methods for creating new values, and for assessing the class of value.

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

	// enum fields within the switch can be switched on directly.
	switch animal {
	case Animals.Cat:
	case Animals.Dog:
	default:
	}

Switch values have builtin support for JSON marshaling and unmarshaling. The behaviour of this can
be controlled with json tags. [Iota]-backed values are marshaled as strings, switches with variable
[Case] values will be boxed into an JSON object with a type discriminator.

Note that switch types do not restrict the underlying memory representation to the set of values
defined in the switch type, so a default case should be included for any switch statements on the
value of a switch type.
*/
package xyz

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// Switch on the underlying storage in order
// to represent a restricted set of values.
// Can be used as the underlying value for
// a named type. Each case must be compatible
// with the memory storage representation.
type Switch[Storage any, Values any] struct {
	switchMethods[Storage, Values] // export methods.
}

// Iota can be used to flag the storage of a switch as
// only containing enumerated values.
type Iota struct{}

type varWith[Storage any, Values any] interface {
	~struct {
		switchMethods[Storage, Values]
	}
	variant()
	Values(internal) Values
}

// Raw returns a switch value of the given type, with the given storage.
func Raw[Variant varWith[Storage, Values], Storage any, Values any](val Storage) Variant {
	var zero Variant
	raw := (struct {
		switchMethods[Storage, Values]
	})(zero)
	raw.ram = val
	return Variant(raw)
}

// AccessorFor returns an accessor for the given switch type. Call this using the
// typename.Values, ie. if the switch type is named MyType, pass MyType.Values to
// this function.
func AccessorFor[S any, T any, V func(S, internal) T](values V) T {
	var zero S
	return values(zero, internal{})
}

// ValueOf returns the value of the switch. Typically used as the expression in a switch statement.
func ValueOf[Storage any, Values any, Variant varWith[Storage, Values]](variant Variant) TypeOf[Variant] {
	a := (struct {
		switchMethods[Storage, Values]
	})(variant).tag
	wrappable, ok := reflect.Zero(a.ctyp).Interface().(interface{ wrap(*accessor) any })
	if !ok {
		return nil
	}
	return wrappable.wrap(a).(TypeOf[Variant])
}

// TypeOf represents the type of a field within a variant.
type TypeOf[T any] interface {
	fmt.Stringer

	value() T
}

// switchMethods can be embedded into a struct to
// provide methods for interacting with a variant.
type switchMethods[Storage any, Values any] struct {
	tag *accessor
	ram Storage
}

func (v switchMethods[Storage, Values]) Get() (Storage, bool) {
	return v.ram, v.tag != nil
}

// String implements [fmt.Stringer].
func (v switchMethods[Storage, Values]) String() string {
	access := v.tag
	if access.text != "" || access.zero {
		if access.fmts {
			return fmt.Sprintf(access.text, access.get(&v))
		}
		return access.text
	}
	if access.void {
		return access.name
	}
	return fmt.Sprint(access.get(&v))
}

func (switchMethods[Storage, Values]) Validate(val any) bool {
	if reflect.TypeOf(val) != reflect.TypeOf([0]Storage{}).Elem() {
		return false
	}
	return false
}

func (v switchMethods[Storage, Values]) variant() {}

func (v *switchMethods[Storage, Values]) storage() (any, *accessor) {
	return &v.ram, v.tag
}

func (v *switchMethods[Storage, Values]) setTag(tag *accessor) {
	v.tag = tag
}

func (v switchMethods[Storage, Values]) typeOf(field reflect.StructField) reflect.Type {
	type isVary interface {
		vary() reflect.Type
	}
	if field.Type.Implements(reflect.TypeOf([0]isVary{}).Elem()) {
		return reflect.Zero(field.Type).Interface().(isVary).vary()
	}
	if field.Type.Kind() == reflect.Struct && field.Type.NumField() > 0 && field.Type.Field(0).Type == reflect.TypeOf(v) {
		return nil
	}
	panic(fmt.Sprintf("invalid variant field: %s", field.Type))
}

type internal struct{}

func (v switchMethods[Storage, Values]) Values(internal) Values {
	var zero Values
	var rtype = reflect.TypeOf(zero)
	var rvalue = reflect.ValueOf(&zero).Elem()
	var stype = reflect.TypeOf([0]Storage{}).Elem()
	var sptrs = hasPointers(stype)
	for i := 0; i < rtype.NumField(); i++ {
		if i > math.MaxUint8 {
			panic("too many variant values")
		}
		field := rtype.Field(i)
		text, hasText := field.Tag.Lookup("text")
		if !hasText && stype.Kind() == reflect.String {
			panic(fmt.Sprintf("missing text tag for string variant field '%s'", field.Name))
		}
		enum := uint64(0)
		if s, ok := field.Tag.Lookup("enum"); ok {
			u, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				panic(fmt.Sprintf("invalid enum tag '%s': %s", field.Tag.Get("enum"), err))
			}
			enum = u
		} else {
			enum = uint64(i)
		}
		void := false
		safe := false
		ftype := v.typeOf(field)
		if ftype == nil {
			void = true
		} else {
			ptrs := hasPointers(ftype)
			safe = stype.Kind() == reflect.Interface || stype.Kind() == reflect.UnsafePointer ||
				stype.Kind() == reflect.String || (stype.Kind() == reflect.Slice && !ptrs) || (stype.Size() >= ftype.Size() && !sptrs && !ptrs)
		}
		access := &accessor{
			name: field.Name,
			enum: enum,
			void: void,
			text: text,
			zero: text == "" && hasText,
			fmts: strings.Contains(text, "%"),
			safe: safe || void,
			ctyp: field.Type,
			rtyp: ftype,
		}
		if !access.void {
			if !safe {
				panic(fmt.Sprintf("unsafe use of variant accessor '%s': incompatible with storage", field.Name))
			}
			type settable interface {
				set(*accessor)
			}
			rvalue.Field(i).Addr().Interface().(settable).set(access)
		}
	}
	return zero
}

type isVariant interface {
	variant()
}

type hasStorage interface {
	storage() (any, *accessor)
	setTag(*accessor)
}

type accessor struct {
	void bool
	fmts bool
	zero bool // is a zero value
	safe bool
	enum uint64
	name string
	text string
	ctyp reflect.Type
	rtyp reflect.Type
}

func (v *accessor) get(ram any) any {
	if !v.safe {
		panic("unintialized variant")
	}
	storage, check := any(ram).(hasStorage).storage()
	if check != v {
		panic("variant access violation")
	}
	var (
		rvalue = reflect.ValueOf(storage).Elem()
	)
	switch rvalue.Kind() {
	case reflect.String:
		var s = reflect.New(v.rtyp).Elem()
		fmt.Sscanf(rvalue.String(), v.text, s.Addr().Interface())
		return s.Interface()
	case reflect.Slice, reflect.UnsafePointer:
		return reflect.NewAt(v.rtyp, rvalue.UnsafePointer()).Elem().Interface()
	case reflect.Interface:
		return rvalue.Interface()
	default:
		return reflect.NewAt(v.rtyp, rvalue.Addr().UnsafePointer()).Elem().Interface()
	}
}

// as is unsafe,
func (v *accessor) as(ram any, val any) {
	if !v.safe {
		panic("unintialized variant")
	}
	if reflect.TypeOf(val) != v.rtyp {
		panic("unsafe use of variant accessor")
	}
	storage, _ := any(ram).(hasStorage).storage()
	var (
		rvalue = reflect.ValueOf(storage).Elem()
	)
	switch rvalue.Kind() {
	case reflect.Bool:
		if v.enum > 0 {
			rvalue.SetBool(true)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		rvalue.SetInt(int64(v.enum))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		rvalue.SetUint(v.enum)
	case reflect.Float32, reflect.Float64:
		rvalue.SetFloat(float64(v.enum))
	case reflect.Complex64, reflect.Complex128:
		rvalue.SetComplex(complex(float64(v.enum), 0))
	case reflect.Array:
		reflect.NewAt(reflect.TypeOf(val), rvalue.Addr().UnsafePointer()).Elem().Set(reflect.ValueOf(val))
	case reflect.String:
		if v.fmts {
			rvalue.SetString(fmt.Sprintf(v.text, val))
		} else {
			rvalue.SetString(v.text)
		}
	case reflect.Interface:
		rvalue.Set(reflect.ValueOf(val))
	case reflect.Slice:
		var length = int(reflect.TypeOf(val).Size() / rvalue.Type().Elem().Size())
		rvalue.Set(reflect.MakeSlice(rvalue.Type(), length, length))
		reflect.NewAt(reflect.TypeOf(val), rvalue.UnsafePointer()).Set(reflect.ValueOf(val))
	case reflect.Struct:
		reflect.NewAt(reflect.TypeOf(val), rvalue.Addr().UnsafePointer()).Set(reflect.ValueOf(val))
	case reflect.UnsafePointer:
		value := reflect.ValueOf(val)
		if !value.CanAddr() {
			copy := reflect.New(value.Type()).Elem()
			copy.Set(value)
			value = copy
		}
		rvalue.SetPointer(value.Addr().UnsafePointer())
	default:
		panic("unreachable")
	}
	any(ram).(hasStorage).setTag(v)
}

// Case indicates that a value within a variant can vary
// in value, constrained by a particular type.
type Case[Variant isVariant, Constraint any] struct {
	_        [0]*Variant
	_        [0]*Constraint
	accessor *accessor
}

func (v *Case[Variant, Constraint]) set(to *accessor) {
	v.accessor = to
}

func (Case[Variant, Constraint]) wrap(as *accessor) any {
	return Case[Variant, Constraint]{accessor: as}
}

func (v Case[Variant, Constraint]) value() Variant {
	var zero Variant
	return zero
}

func (v Case[Variant, Constraint]) vary() reflect.Type {
	return reflect.TypeOf([0]Constraint{}).Elem()
}

// As returns the value of the variant as the given type.
func (v Case[Variant, Constraint]) As(val Constraint) Variant {
	var zero Variant
	v.accessor.as(&zero, val)
	return zero
}

func (v Case[Variant, Constraint]) String() string {
	return v.accessor.name
}

// Get returns the value of the variant as the given type.
func (v Case[Variant, Constraint]) Get(variant Variant) Constraint {
	return v.accessor.get(&variant).(Constraint)
}

func hasPointers(value reflect.Type) bool {
	if value == nil || value.Size() == 0 {
		return false
	}
	switch value.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return false
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if hasPointers(value.Field(i).Type) {
				return true
			}
		}
		return false
	case reflect.Array:
		return hasPointers(value.Elem())

	}
	return true
}

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
		return err
	}
	clear(*o)
	if *o == nil {
		*o = New(val)
	} else {
		(*o)[ok{}] = val
	}
	return nil
}
