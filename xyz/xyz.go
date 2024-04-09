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

# Switch Types

Switch types are used to represent a discriminated set of values.

To represent an enumerated type (enum) where each value must be distinct you can add
fields to the switch type with the same type as the switch itself.

	type Animal xyz.Switch[xyz.Iota, struct {
		Cat Animal
		Dog Animal
	}]

Union types can also be represented, where each switch case can have a variable value.

	type MyValue xyz.Switch[any, struct {
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

In order to create a new switch value, or to assess the value of a switch, you must
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

Switch values have builtin support for JSON marshaling and unmarshaling. The behaviour
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

Note that switch types do not restrict the underlying value in memory to the set
of values defined in the switch type, so a default case should be included for any
switch statements on the value of a switch type.

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
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"runtime.link/api/xray"
)

// Switch on the underlying storage in order
// to represent a restricted set of values.
// Can be used as the underlying value for
// a named type. Each case must be compatible
// with the memory storage representation.
type Switch[Storage any, Values any] struct {
	switchMethods[Storage, Values] // export methods.
}

// Enum can be used to flag the storage of a switch as
// only containing enumerated values.
type Enum struct{}

type varWith[Storage any, Values any] interface {
	~struct {
		switchMethods[Storage, Values]
	}
	variant() *accessor
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
	if a == nil {
		return nil
	}
	wrappable, ok := reflect.Zero(a.ctyp).Interface().(interface{ wrap(*accessor) any })
	if !ok {
		return nil
	}
	wrapped := wrappable.wrap(a)
	if reflect.TypeOf(wrapped) == a.ctyp {
		return wrapped.(TypeOf[Variant])
	}
	return reflect.ValueOf(wrapped).Convert(a.ctyp).Interface().(TypeOf[Variant])
}

// TypeOf represents the type of a field within a variant.
type TypeOf[T any] interface {
	fmt.Stringer

	Key() (string, error)

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
	if v.tag == nil {
		return ""
	}
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

// MarshalPair based on the json field/type name and the storage value.
func (v switchMethods[Storage, Values]) MarshalPair() (Pair[string, Storage], error) {
	var zero Storage
	access := v.tag
	if access.text != "" || access.zero {
		if access.fmts {
			return NewPair(fmt.Sprintf(access.text, access.get(&v)), zero), nil
		}
		return NewPair(access.text, zero), nil
	}
	if access.json == "" {
		return Pair[string, Storage]{}, errors.New("MarshalPair requires a json tag for the '" + access.name + "' Case")
	}
	base, _, _ := strings.Cut(access.json, ",")
	if base == "" {
		base = access.name
	}
	name, rule, _ := strings.Cut(base, "?")
	key, val, _ := strings.Cut(rule, "=")

	if key != "" && val != "" {
		return NewPair(val, v.ram), nil
	}
	return NewPair(name, v.ram), nil
}

// UnmarshalPair based on the json field/type name and the storage value.
func (v *switchMethods[Storage, Values]) UnmarshalPair(pair Pair[string, Storage]) error {
	s, storage := pair.Split()

	accessors := v.accessors()
	for i, access := range accessors {
		if access.text != "" || access.zero {
			if access.text == s {
				v.tag = accessors[i]
				v.ram = storage
				return nil
			}
		}
		if access.json == "" {
			continue
		}
		base, _, _ := strings.Cut(access.json, ",")
		if base == "" {
			base = access.name
		}
		name, rule, _ := strings.Cut(base, "?")
		key, val, _ := strings.Cut(rule, "=")
		if key != "" && val != "" {
			if val == s {
				v.tag = accessors[i]
				v.ram = storage
				return nil
			}
		}
		if name == s {
			v.tag = accessors[i]
			v.ram = storage
			return nil
		}
	}
	return errors.New("no matching cases found for '" + s + "'")
}

func (v switchMethods[Storage, Values]) MarshalJSON() ([]byte, error) {
	access := v.tag
	if access == nil {
		if reflect.TypeOf(v.ram) == reflect.TypeOf(json.RawMessage{}) {
			return any(v.ram).(json.RawMessage), nil
		}
		return []byte("null"), nil
	}
	if access.text != "" || access.zero {
		if access.fmts {
			return json.Marshal(fmt.Sprintf(access.text, access.get(&v)))
		}
		return json.Marshal(access.text)
	}
	base, kind, _ := strings.Cut(access.json, ",")
	if kind == "null" {
		return []byte("null"), nil
	}
	name, rule, _ := strings.Cut(base, "?")
	key, val, _ := strings.Cut(rule, "=")
	if name != "" {
		wrapper := map[string]any{
			name: access.get(&v),
		}
		if key != "" {
			wrapper[key] = val
		}
		return json.Marshal(wrapper)
	}
	if key != "" {
		merged := reflect.New(reflect.StructOf([]reflect.StructField{
			{
				Name:      "UnionValue",
				Anonymous: true,
				Type:      access.rtyp,
			},
			{
				Name: "UnionType",
				Tag:  reflect.StructTag(`json:"` + key + `"`),
				Type: reflect.TypeOf(val),
			},
		})).Elem()
		merged.Field(0).Set(reflect.ValueOf(access.get(&v)))
		merged.Field(1).Set(reflect.ValueOf(val))
		return json.Marshal(merged.Interface())
	}
	return json.Marshal(v.tag.get(&v))
}

func (v *switchMethods[Storage, Values]) UnmarshalJSON(data []byte) error {
	unmarshal := func(data []byte) error {
		ptr := reflect.New(v.tag.rtyp)
		if err := json.Unmarshal(data, ptr.Interface()); err != nil {
			return err
		}
		v.tag.as(v, ptr.Elem().Interface())
		return nil
	}

	accessors := v.accessors()
	for i, access := range accessors {
		if access.text != "" || access.zero {
			var s string
			if err := json.Unmarshal(data, &s); err != nil {
				return xray.Error(err)
			}
			if access.text == s {
				v.tag = accessors[i]
				return nil
			}
			continue
		}
		if len(data) == 0 {
			continue
		}
		if access.json == "" {
			continue
		}
		base, kind, _ := strings.Cut(access.json, ",")
		name, rule, _ := strings.Cut(base, "?")
		key, val, _ := strings.Cut(rule, "=")
		switch kind {
		case "string":
			if data[0] != '"' {
				continue
			}
		case "number":
			if data[0] != '-' && (data[0] < '0' || data[0] > '9') {
				continue
			}
		case "object":
			if data[0] != '{' {
				continue
			}
		case "array":
			if data[0] != '[' {
				continue
			}
		case "null":
			if string(data) == "null" {
				v.tag = accessors[i]
				return nil
			}
			continue
		}
		if base == "" {
			v.tag = accessors[i]
			return unmarshal(data)
		}
		var decoded = make(map[string]json.RawMessage)
		if err := json.Unmarshal(data, &decoded); err != nil {
			return err
		}
		if rule == "" {
			if val, ok := decoded[name]; ok {
				v.tag = accessors[i]
				return unmarshal(val)
			}
			continue
		}
		if name == "" {
			v.tag = accessors[i]
			return unmarshal(data)
		}
		if string(decoded[key]) == strconv.Quote(val) {
			v.tag = accessors[i]
			return unmarshal(decoded[name])
		}
	}
	if reflect.TypeOf([0]Storage{}).Elem() == reflect.TypeOf([0]any{}).Elem() {
		reflect.ValueOf(&v.ram).Elem().Set(reflect.ValueOf(json.RawMessage(data)))
		return nil
	}
	return json.Unmarshal(data, &v.ram)
}

func (v switchMethods[Storage, Values]) MarshalText() ([]byte, error) {
	access := v.tag
	if access == nil {
		return []byte{}, nil
	}
	if access.text != "" || access.zero {
		if access.fmts {
			return []byte(fmt.Sprintf(access.text, access.get(&v))), nil
		}
		return []byte(access.text), nil
	}
	return nil, errors.New("cannot marshal non-text variant")
}

func (v *switchMethods[Storage, Values]) UnmarshalText(data []byte) error {
	accessors := v.accessors()
	for i, access := range accessors {
		if access.text == string(data) {
			v.tag = accessors[i]
			return nil
		}
	}
	return nil
}

func (switchMethods[Storage, Values]) Validate(val any) bool {
	if reflect.TypeOf(val) != reflect.TypeOf([0]Storage{}).Elem() {
		return false
	}
	return false
}

func (v switchMethods[Storage, Values]) append(impl *accessor) {
	mutex.Lock()
	defer mutex.Unlock()
	if slice, ok := cache[reflect.TypeOf(v)]; ok {
		for _, access := range slice {
			if access == impl {
				return
			}
		}
		cache[reflect.TypeOf(v)] = append(slice, impl)
	} else {
		cache[reflect.TypeOf(v)] = []*accessor{impl}
	}
}

func (v switchMethods[Storage, Values]) raw() Storage {
	return v.ram
}

func (v switchMethods[Storage, Values]) variant() *accessor {
	return v.tag
}

func (v *switchMethods[Storage, Values]) storage() (any, *accessor) {
	return &v.ram, v.tag
}

func (v *switchMethods[Storage, Values]) set(tag *accessor) { v.tag = tag }

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
	if method, ok := field.Type.MethodByName("Values"); ok &&
		method.Type.NumIn() == 2 && method.Type.NumOut() == 1 && method.Type.Out(0) == reflect.TypeOf([0]Values{}).Elem() {
		return nil
	}
	panic(fmt.Sprintf("invalid variant field: %s", field.Type))
}

type internal struct{}

var mutex sync.RWMutex
var cache = make(map[reflect.Type][]*accessor)

func (v switchMethods[Storage, Values]) accessors() (slice []*accessor) {
	slice, ok := func() ([]*accessor, bool) {
		mutex.RLock()
		defer mutex.RUnlock()
		if slice, ok := cache[reflect.TypeOf(v)]; ok {
			return slice, true
		}
		return nil, false
	}()
	if ok {
		return slice
	}
	var zero Values
	var rtype = reflect.TypeOf(zero)
	var stype = reflect.TypeOf([0]Storage{}).Elem()
	var sptrs = hasPointers(stype)
	for i := 0; i < rtype.NumField(); i++ {
		if i > math.MaxUint8 {
			panic("too many variant values")
		}
		field := rtype.Field(i)
		text, hasText := field.Tag.Lookup("txt")
		if !hasText && stype.Kind() == reflect.String {
			panic(fmt.Sprintf("missing text tag for string variant field '%s'", field.Name))
		}
		enum := uint64(0)
		if s, ok := field.Tag.Lookup("xyz"); ok {
			u, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				panic(fmt.Sprintf("invalid enum tag '%s': %s", field.Tag.Get("xyz"), err))
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
		slice = append(slice, &accessor{
			name: field.Name,
			enum: enum,
			void: void,
			text: text,
			json: field.Tag.Get("json"),
			zero: text == "" && hasText,
			fmts: strings.Contains(text, "%"),
			safe: safe || void,
			ctyp: field.Type,
			rtyp: ftype,
		})
	}
	mutex.Lock()
	cache[reflect.TypeOf(v)] = slice
	mutex.Unlock()
	return slice
}

func (v switchMethods[Storage, Values]) Values(internal) Values {
	var zero Values
	var rvalue = reflect.ValueOf(&zero).Elem()
	var rtype = reflect.TypeOf(zero)
	accessors := v.accessors()
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		access := accessors[i]
		if !access.safe {
			panic(fmt.Sprintf("unsafe use of variant accessor '%s': incompatible with storage", field.Name))
		}
		type settable interface {
			set(*accessor)
		}
		rvalue.Field(i).Addr().Interface().(settable).set(accessors[i])
	}
	return zero
}

type isVariant interface {
	variant() *accessor
	append(*accessor)
}

type isVariantWith[Storage any] interface {
	variant() *accessor
	append(*accessor)
	raw() Storage
}

type hasStorage interface {
	storage() (any, *accessor)
	set(*accessor)
}

type accessor struct {
	void bool
	fmts bool
	zero bool // is a zero value
	safe bool
	enum uint64
	name string
	text string
	json string
	ctyp reflect.Type
	rtyp reflect.Type
	pack *packing
}

func (access *accessor) key() (string, error) {
	if access.text != "" || access.zero {
		return access.text, nil
	}
	if access.json == "" {
		return "", errors.New(access.ctyp.String() + ": requires a json tag for the '" + access.name + "' Case")
	}
	base, _, _ := strings.Cut(access.json, ",")
	if base == "" {
		base = access.name
	}
	name, rule, _ := strings.Cut(base, "?")
	key, val, _ := strings.Cut(rule, "=")
	if key != "" && val != "" {
		return val, nil
	}
	return name, nil
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
	storage, _ := any(ram).(hasStorage).storage()
	var (
		rvalue = reflect.ValueOf(storage).Elem()
	)
	if rvalue.Kind() != reflect.Interface && reflect.TypeOf(val) != v.rtyp {
		panic("unsafe use of variant accessor")
	}
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
	any(ram).(hasStorage).set(v)
}

// Case indicates that a value within a variant can vary
// in value, constrained by a particular type.
type Case[Variant isVariant, Constraint any] struct {
	caseMethods[Variant, Constraint]
}

type caseMethods[Variant isVariant, Constraint any] struct {
	_        [0]*Variant
	_        [0]*Constraint
	accessor *accessor
}

func (v *caseMethods[Variant, Constraint]) set(to *accessor) {
	v.accessor = to

	var parent Variant
	parent.append(to)
}

func (caseMethods[Variant, Constraint]) wrap(as *accessor) any {
	return Case[Variant, Constraint]{caseMethods[Variant, Constraint]{accessor: as}}
}

func (v caseMethods[Variant, Constraint]) value() Variant {
	var zero Variant
	return zero
}

func (v caseMethods[Variant, Constraint]) vary() reflect.Type {
	return reflect.TypeOf([0]Constraint{}).Elem()
}

// As returns the value of the variant as the given type.
func (v caseMethods[Variant, Constraint]) As(val Constraint) Variant {
	var zero Variant
	v.accessor.as(&zero, val)
	return zero
}

func (v caseMethods[Variant, Constraint]) New(val Constraint) Variant  { return v.As(val) }
func (v caseMethods[Variant, Constraint]) With(val Constraint) Variant { return v.As(val) }

// Key returns the key for this case, as if it were returned by MarshalPair.
func (v caseMethods[Variant, Constraint]) Key() (string, error) {
	return v.accessor.key()
}

func (v caseMethods[Variant, Constraint]) String() string {
	return v.accessor.name
}

// Get returns the value of the variant as the given type.
func (v caseMethods[Variant, Constraint]) Get(variant Variant) Constraint {
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
		return xray.Error(err)
	}
	clear(*o)
	if *o == nil {
		*o = New(val)
	} else {
		(*o)[ok{}] = val
	}
	return nil
}

// CaseReflection is equivalent to a [reflect.StructField] for switch types.
type CaseReflection struct {
	Name string
	Tags reflect.StructTag
	Docs string
	Vary reflect.Type
	Test func(any) bool
}

func (v switchMethods[Storage, Values]) Reflection() []CaseReflection {
	var zero Values
	var rtype = reflect.TypeOf(zero)
	var cases []CaseReflection
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		access := v.accessors()[i]
		cases = append(cases, CaseReflection{
			Name: field.Name,
			Tags: field.Tag,
			Docs: access.text,
			Vary: v.typeOf(field),
			Test: func(a any) bool {
				variant, ok := a.(isVariant)
				if ok {
					accessr := variant.variant()
					if accessr != nil {
						return access == accessr
					}
				}
				return false
			},
		})
	}
	return cases
}
