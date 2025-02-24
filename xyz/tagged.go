package xyz

import (
	"encoding"
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

// Tagged union with the underlying storage in order
// to represent a restricted set of values.
// Can be used as the underlying value for
// a named type. Each case must be compatible
// with the memory storage representation.
type Tagged[Storage any, Values any] struct {
	taggedMethods[Storage, Values] // export methods.
}

type taggedWith[Storage any, Values any] interface {
	~struct {
		taggedMethods[Storage, Values]
	}
	variant() *accessor
	Values(internal) Values
}

// AccessorFor retu an accessor for the given switch type. Call this using the
// typename.Values, ie. if the switch type is named MyType, pass MyType.Values to
// this function.
func AccessorFor[S any, T any, V func(S, internal) T](values V) T {
	var zero S
	return values(zero, internal{})
}

// ValueOf returns the value of the switch. Typically used as the expression in a switch statement.
func ValueOf[Storage any, Values any, Variant taggedWith[Storage, Values]](variant Variant) TypeOf[Variant] {
	a := (struct {
		taggedMethods[Storage, Values]
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

// taggedMethods can be embedded into a struct to
// provide methods for interacting with a variant.
type taggedMethods[Storage any, Values any] struct {
	tag *accessor
	ram Storage
}

func (v taggedMethods[Storage, Values]) Get() (Storage, bool) {
	return v.ram, v.tag != nil
}

func (v taggedMethods[Storage, Values]) Interface() any { return v.tag.get(&v) }

// String implements [fmt.Stringer].
func (v taggedMethods[Storage, Values]) String() string {
	if v.tag == nil {
		return ""
	}
	access := v.tag
	if access.text != "" || access.zero {
		if access.fmts {
			return fmt.Sprintf(access.text, expand(access.get(&v))...)
		}
		return access.text
	}
	if access.void {
		return access.name
	}
	return fmt.Sprint(access.get(&v))
}

// MarshalPair based on the json field/type name and the storage value.
func (v taggedMethods[Storage, Values]) MarshalPair() (Pair[string, Storage], error) {
	var zero Storage
	access := v.tag
	if access.text != "" || access.zero {
		if access.fmts {
			return NewPair(fmt.Sprintf(access.text, expand(access.get(&v))...), zero), nil
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
func (v *taggedMethods[Storage, Values]) UnmarshalPair(pair Pair[string, Storage]) error {
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

func (v taggedMethods[Storage, Values]) MarshalJSON() ([]byte, error) {
	access := v.tag
	if access == nil {
		if reflect.TypeOf(v.ram) == reflect.TypeOf(json.RawMessage{}) {
			return any(v.ram).(json.RawMessage), nil
		}
		return []byte("null"), nil
	}
	if access.text != "" || access.zero {
		if access.fmts {
			return json.Marshal(fmt.Sprintf(access.text, expand(access.get(&v))...))
		}
		return json.Marshal(access.text)
	}
	base, kind, _ := strings.Cut(access.json, ",")
	if kind == "null" {
		return []byte("null"), nil
	}
	name, rule, _ := strings.Cut(base, "?")
	key, val, hasConst := strings.Cut(rule, "=")
	if name != "" {
		wrapper := map[string]any{
			name: access.get(&v),
		}
		if key != "" {
			wrapper[key] = val
		}
		return json.Marshal(wrapper)
	}
	if hasConst {
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

func (v *taggedMethods[Storage, Values]) UnmarshalJSON(data []byte) error {
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
				return xray.New(err)
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
		name, rule, hasKey := strings.Cut(base, "?")
		key, val, hasConst := strings.Cut(rule, "=")
		switch kind {
		case "string":
			if access.rtyp.Kind() == reflect.String && data[0] != '"' {
				continue
			}
			if access.rtyp.Kind() != reflect.String {
				unmarshal = func(data []byte) error {
					ptr := reflect.New(v.tag.rtyp)
					switch ptr := ptr.Interface().(type) {
					case json.Unmarshaler:
						if err := json.Unmarshal(data, &ptr); err != nil {
							return xray.New(err)
						}
					case encoding.TextUnmarshaler:
						var s string
						if err := json.Unmarshal(data, &s); err != nil {
							return xray.New(err)
						}
						if err := ptr.UnmarshalText([]byte(s)); err != nil {
							return xray.New(err)
						}
					default:
						var s string
						if err := json.Unmarshal(data, &s); err != nil {
							return xray.New(err)
						}
						if _, err := fmt.Sscan(s, ptr); err != nil {
							return xray.New(err)
						}
					}
					v.tag.as(v, ptr.Elem().Interface())
					return nil
				}
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
				return xray.New(unmarshal(val))
			}
			continue
		}
		keyValue, keyExists := decoded[key]
		if (hasKey && name == "") || (!hasConst && keyExists) {
			v.tag = accessors[i]
			return xray.New(unmarshal(data))
		}
		if hasKey && hasConst && string(keyValue) == strconv.Quote(val) {
			v.tag = accessors[i]
			return xray.New(unmarshal(decoded[name]))
		}
	}
	if reflect.TypeOf([0]Storage{}).Elem() == reflect.TypeOf([0]any{}).Elem() {
		reflect.ValueOf(&v.ram).Elem().Set(reflect.ValueOf(json.RawMessage(data)))
		return nil
	}
	return json.Unmarshal(data, &v.ram)
}

func (v taggedMethods[Storage, Values]) MarshalText() ([]byte, error) {
	access := v.tag
	if access == nil {
		return []byte{}, nil
	}
	if access.text != "" || access.zero {
		if access.fmts {
			return []byte(fmt.Sprintf(access.text, expand(access.get(&v))...)), nil
		}
		return []byte(access.text), nil
	}
	return nil, errors.New("cannot marshal non-text variant")
}

func (v *taggedMethods[Storage, Values]) UnmarshalText(data []byte) error {
	accessors := v.accessors()
	for i, access := range accessors {
		if access.text == string(data) {
			v.tag = accessors[i]
			return nil
		}
	}
	return nil
}

func (taggedMethods[Storage, Values]) Validate(val any) bool {
	if reflect.TypeOf(val) != reflect.TypeOf([0]Storage{}).Elem() {
		return false
	}
	return false
}

func (v taggedMethods[Storage, Values]) append(impl *accessor) {
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

func (v taggedMethods[Storage, Values]) raw() Storage {
	return v.ram
}

func (v taggedMethods[Storage, Values]) variant() *accessor {
	return v.tag
}

func (v *taggedMethods[Storage, Values]) storage() (any, *accessor) {
	return &v.ram, v.tag
}

func (v *taggedMethods[Storage, Values]) set(tag *accessor) { v.tag = tag }

func (v taggedMethods[Storage, Values]) typeOf(field reflect.StructField) reflect.Type {
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

func (v taggedMethods[Storage, Values]) accessors() (slice []*accessor) {
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
			rtag: field.Tag,
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

func (v taggedMethods[Storage, Values]) Tag() reflect.StructTag {
	if v.tag == nil {
		return ""
	}
	return v.tag.rtag
}

func (v taggedMethods[Storage, Values]) Values(internal) Values {
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
	rtag reflect.StructTag
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
	if v.rtyp == nil {
		return nil
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
			rvalue.SetString(fmt.Sprintf(v.text, expand(val)...))
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

// CaseReflection is equivalent to a [reflect.StructField] for switch types.
type CaseReflection struct {
	Name string
	Tags reflect.StructTag
	Vary reflect.Type
	Test func(any) bool
}

func (v taggedMethods[Storage, Values]) Reflection() []CaseReflection {
	var zero Values
	var rtype = reflect.TypeOf(zero)
	var cases []CaseReflection
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		access := v.accessors()[i]
		cases = append(cases, CaseReflection{
			Name: field.Name,
			Tags: field.Tag,
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
