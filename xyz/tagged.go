package xyz

import (
	"bytes"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/iqhive/runtime.link/api/xray"
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

	ValuesJSON() (oneof []json.RawMessage)

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

func (v taggedMethods[Storage, Values]) IsZero() bool { return v.tag == nil }

func (v taggedMethods[Storage, Values]) Interface() any {
	if v.tag == nil {
		return nil
	}
	return v.tag.get(&v)
}

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
	if access == nil {
		return Pair[string, Storage]{}, errors.New("MarshalPair requires a value with a Case")
	}
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
		return marshalMerged(access.get(&v), key, val)
	}
	return json.Marshal(v.tag.get(&v))
}

// marshalMerged marshals a case selected by a discriminator, adding the key to
// the fields of the value. A value that is not a JSON object has no fields to
// add it to, so it sits under "value", the same shape [taggedMethods.TypesJSON]
// describes. The value's own JSON is worked on, rather than a struct embedding
// the value, as not every type can be embedded and a type with a MarshalJSON
// method would have that method promoted over the discriminator.
func marshalMerged(value any, key, val string) ([]byte, error) {
	discriminator, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return json.Marshal(map[string]json.RawMessage{key: discriminator})
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil || fields == nil {
		return json.Marshal(map[string]json.RawMessage{key: discriminator, "value": body})
	}
	if _, clash := fields[key]; clash {
		fields[key] = discriminator
		return json.Marshal(fields)
	}
	name, err := json.Marshal(key)
	if err != nil {
		return nil, err
	}
	// Added to the end of the object, so that the value keeps its field order.
	merged := body[:len(body)-1]
	if len(fields) > 0 {
		merged = append(merged, ',')
	}
	merged = append(merged, name...)
	merged = append(merged, ':')
	merged = append(merged, discriminator...)
	return append(merged, '}'), nil
}

// UnmarshalJSON decodes the case that data selects. v is only written once the
// whole of a case has decoded, so that a failure leaves it as it was, and every
// outcome replaces both its tag and its storage, because a tag left over the
// storage of another case makes a later Get panic on JSON that any client sends.
func (v *taggedMethods[Storage, Values]) UnmarshalJSON(data []byte) error {
	unmarshal := func(access *accessor, data []byte) error {
		ptr := reflect.New(access.rtyp)
		if err := json.Unmarshal(data, ptr.Interface()); err != nil {
			return err
		}
		return v.setDecoded(access, ptr.Elem().Interface())
	}
	decode := func(access *accessor, data []byte) error {
		if access.void {
			return v.setCase(access)
		}
		return unmarshal(access, data)
	}
	accessors := v.accessors()
	for i, access := range accessors {
		if access.text != "" || access.zero {
			var s string
			if err := json.Unmarshal(data, &s); err != nil {
				return xray.New(err)
			}
			if access.text == s {
				return v.setCase(accessors[i])
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
		key, val, hasConst := strings.Cut(rule, "=")
		switch kind {
		case "string":
			isString := access.void || access.rtyp.Kind() == reflect.String
			if isString && data[0] != '"' {
				continue
			}
			if !isString {
				unmarshal = func(access *accessor, data []byte) error {
					ptr := reflect.New(access.rtyp)
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
							return xray.New(fmt.Errorf("failed to unmarshal '%s' into %T: %w", s, ptr, err))
						}
					}
					return v.setDecoded(access, ptr.Elem().Interface())
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
				return v.setCase(accessors[i])
			}
			continue
		}
		if base == "" {
			return decode(accessors[i], data)
		}
		var decoded = make(map[string]json.RawMessage)
		if err := json.Unmarshal(data, &decoded); err != nil {
			return err
		}
		if rule == "" {
			if val, ok := decoded[name]; ok {
				return xray.New(decode(accessors[i], val))
			}
			continue
		}
		// The rule picks this case by another key of the object:
		// "?key" when it is present, "?key=val" when it holds the
		// string val. The case's value is the named field, or the
		// whole object when the tag names none. A case whose rule does
		// not match must fall through to the next, whatever its order.
		keyValue, keyExists := decoded[key]
		if !keyExists {
			continue
		}
		if hasConst {
			var s string
			if json.Unmarshal(keyValue, &s) != nil || s != val {
				continue
			}
		}
		if name != "" {
			return xray.New(decode(accessors[i], decoded[name]))
		}
		err := decode(accessors[i], data)
		// The whole object can only fail where the value was not an object
		// when it was merged, see [marshalMerged].
		if value, wrapped := decoded["value"]; err != nil && hasConst && wrapped {
			err = decode(accessors[i], value)
		}
		return xray.New(err)
	}
	var unknown taggedMethods[Storage, Values]
	if reflect.TypeOf([0]Storage{}).Elem() == reflect.TypeOf([0]any{}).Elem() {
		// data belongs to the decoder, which is free to reuse it after this returns.
		reflect.ValueOf(&unknown.ram).Elem().Set(reflect.ValueOf(json.RawMessage(bytes.Clone(data))))
		*v = unknown
		return nil
	}
	if err := json.Unmarshal(data, &unknown.ram); err != nil {
		return err
	}
	*v = unknown
	return nil
}

// setCase makes v the given case with nothing in storage, which is also
// what the accessor of a case without a value holds.
func (v *taggedMethods[Storage, Values]) setCase(access *accessor) error {
	*v = taggedMethods[Storage, Values]{tag: access}
	return nil
}

// setDecoded makes v the given case with a decoded value in storage, or returns
// an error if the storage cannot hold it. [accessor.as] panics in that case,
// which must not be reachable from a decoder.
func (v *taggedMethods[Storage, Values]) setDecoded(access *accessor, val any) error {
	var next taggedMethods[Storage, Values]
	if !access.holds(reflect.TypeOf([0]Storage{}).Elem(), val) {
		return fmt.Errorf("variant case '%s' cannot store a value of type %T", access.name, val)
	}
	access.as(&next, val)
	*v = next
	return nil
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
			return v.setCase(accessors[i])
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
		if field.Name == "_" { // an [Extern] without extensions has no cases.
			continue
		}
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
	defer mutex.Unlock()
	// Concurrent first uses all reach this point. Only one set of accessors
	// can be kept, as [accessor.get] rejects a value tagged by any other,
	// and a set that came first can also hold cases added by an [Extend].
	if first, ok := cache[reflect.TypeOf(v)]; ok {
		return first
	}
	cache[reflect.TypeOf(v)] = slice
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
		if rvalue.IsNil() {
			return reflect.Zero(v.rtyp).Interface()
		}
		return reflect.NewAt(v.rtyp, rvalue.UnsafePointer()).Elem().Interface()
	case reflect.Interface:
		return rvalue.Interface()
	default:
		return reflect.NewAt(v.rtyp, rvalue.Addr().UnsafePointer()).Elem().Interface()
	}
}

// holds reports whether [accessor.as] can store val in the given storage type.
func (v *accessor) holds(storage reflect.Type, val any) bool {
	if !v.safe {
		return false
	}
	if storage.Kind() != reflect.Interface {
		return reflect.TypeOf(val) == v.rtyp
	}
	return val == nil || reflect.TypeOf(val).AssignableTo(storage)
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
		if val == nil {
			rvalue.Set(reflect.Zero(rvalue.Type()))
		} else {
			rvalue.Set(reflect.ValueOf(val))
		}
	case reflect.Slice:
		// Rounded up so that the slice is never smaller than val, and not a
		// zero length slice whose pointer is shared by the whole runtime.
		var size = rvalue.Type().Elem().Size()
		var length = int((reflect.TypeOf(val).Size() + size - 1) / size)
		rvalue.Set(reflect.MakeSlice(rvalue.Type(), length, length))
		reflect.NewAt(reflect.TypeOf(val), rvalue.UnsafePointer()).Elem().Set(reflect.ValueOf(val))
	case reflect.Struct:
		reflect.NewAt(reflect.TypeOf(val), rvalue.Addr().UnsafePointer()).Elem().Set(reflect.ValueOf(val))
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

func (v caseMethods[Variant, Constraint]) ValuesJSON() (oneof []json.RawMessage) {
	var zero Variant
	return caseValuesJSON(reflect.TypeOf(zero))
}

// caseValuesJSON returns the JSON-encoded keys for every case of the variant
// type. Kept non-generic so it compiles once instead of per [Case] instance.
func caseValuesJSON(rtype reflect.Type) (oneof []json.RawMessage) {
	if rtype.Kind() != reflect.Struct || rtype.NumField() == 0 {
		return nil
	}
	mutex.RLock()
	accessors := cache[rtype.Field(0).Type]
	mutex.RUnlock()
	for _, access := range accessors {
		key, err := access.key()
		if err != nil {
			continue
		}
		b, err := json.Marshal(key)
		if err != nil {
			continue
		}
		oneof = append(oneof, b)
	}
	return oneof
}

func (v caseMethods[Variant, Constraint]) String() string {
	return v.accessor.name
}

// Get returns the value of the variant as the given type.
func (v caseMethods[Variant, Constraint]) Get(variant Variant) Constraint {
	// Not a plain assertion: one on a nil interface panics, and JSON null
	// decodes to exactly that for a case of an interface type.
	value, _ := v.accessor.get(&variant).(Constraint)
	return value
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

func (v taggedMethods[Storage, Values]) TypesJSON() []reflect.Type {
	var zero Values
	var rtype = reflect.TypeOf(zero)
	accessors := v.accessors()
	var types []reflect.Type
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		access := accessors[i]
		if access.text != "" || access.zero {
			types = append(types, reflect.TypeOf(""))
			continue
		}
		base, kind, _ := strings.Cut(access.json, ",")
		name, rule, _ := strings.Cut(base, "?")
		key, val, hasConst := strings.Cut(rule, "=")
		valueType := v.typeOf(field)
		if name != "" && !hasConst {
			// Wrapped as {"name": value} with optional discriminator.
			if valueType == nil {
				valueType = reflect.TypeOf(struct{}{})
			}
			fields := []reflect.StructField{
				{
					Name: "Value",
					Type: valueType,
					Tag:  reflect.StructTag(`json:"` + name + `"`),
				},
			}
			if key != "" {
				fields = append(fields, reflect.StructField{
					Name: "Type",
					Type: reflect.TypeOf(""),
					Tag:  reflect.StructTag(`json:"` + key + `"`),
				})
			}
			types = append(types, reflect.StructOf(fields))
			continue
		}
		if hasConst {
			// Discriminated union: value fields merged with {key: val}.
			if valueType == nil {
				valueType = reflect.TypeOf(struct{}{})
			}
			var fields []reflect.StructField
			if valueType.Kind() == reflect.Struct {
				for fi := range valueType.NumField() {
					f := valueType.Field(fi)
					if f.PkgPath != "" {
						continue
					}
					fields = append(fields, f)
				}
			} else {
				fields = append(fields, reflect.StructField{
					Name: "UnionValue",
					Type: valueType,
					Tag:  reflect.StructTag(`json:"value"`),
				})
			}
			fields = append(fields, reflect.StructField{
				Name: "UnionType",
				Type: reflect.TypeOf(val),
				Tag:  reflect.StructTag(`json:"` + key + `"`),
			})
			types = append(types, reflect.StructOf(fields))
			continue
		}
		// Bare value — use the kind hint or the value type directly.
		if valueType == nil {
			valueType = reflect.TypeOf(struct{}{})
		}
		switch kind {
		case "string":
			types = append(types, reflect.TypeOf(""))
		case "number":
			types = append(types, reflect.TypeOf(float64(0)))
		case "object":
			types = append(types, valueType)
		case "array":
			types = append(types, reflect.SliceOf(valueType))
		default:
			types = append(types, valueType)
		}
	}
	return types
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
