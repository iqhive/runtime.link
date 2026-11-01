package xyz

import (
	"bytes"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

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
			return v.formatted()
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
			return NewPair(v.formatted(), zero), nil
		}
		return NewPair(access.text, zero), nil
	}
	if access.json == "" {
		return Pair[string, Storage]{}, errors.New("MarshalPair requires a json tag for the '" + access.name + "' Case")
	}
	key, err := access.pairKey(v.accessors())
	if err != nil {
		return Pair[string, Storage]{}, err
	}
	return NewPair(key, v.ram), nil
}

// UnmarshalPair based on the json field/type name and the storage value. The key
// selects the first case, in the order they are declared, that has the key as its
// text, as the val of its "?key=val" rule, or as its name, which is empty for a
// case with a rule. Failing that, the key of a "?key" rule selects its case, and
// the text that a case with a format writes for a value selects that value.
func (v *taggedMethods[Storage, Values]) UnmarshalPair(pair Pair[string, Storage]) error {
	s, storage := pair.Split()
	accessors := v.accessors()
	access := pairCase(accessors, s)
	if access == nil && s != "" {
		access = presenceCase(accessors, s)
	}
	if access != nil {
		v.tag = access
		v.ram = storage
		return nil
	}
	// MarshalPair leaves the value of a case with a format in its text.
	if matched, err := v.setText(s); matched {
		return err
	}
	return errors.New("no matching cases found for '" + s + "'")
}

// pairCase returns the case that [taggedMethods.UnmarshalPair] selects by its own
// text, val or name, or nil if there is none.
func pairCase(accessors []*accessor, s string) *accessor {
	for _, access := range accessors {
		if (access.text != "" || access.zero) && access.text == s {
			return access
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
		if key != "" && val != "" && val == s {
			return access
		}
		if name == s {
			return access
		}
	}
	return nil
}

// presenceKey returns the key of a "?key" rule, which selects a case when an
// object has that key, or "" for any other case.
func (v *accessor) presenceKey() string {
	base, _, _ := strings.Cut(v.json, ",")
	name, rule, _ := strings.Cut(base, "?")
	key, _, hasConst := strings.Cut(rule, "=")
	if name != "" || hasConst {
		return ""
	}
	return key
}

// presenceCase returns the first case with a "?key" rule for the given key.
func presenceCase(accessors []*accessor, key string) *accessor {
	for _, access := range accessors {
		if access.presenceKey() == key {
			return access
		}
	}
	return nil
}

// pairKey returns the key of a case in a pair, see [taggedMethods.MarshalPair].
// A "?key" case has no name, and its key was "", which UnmarshalPair reads as the
// first case without a name. That stays the key of the first "?key" case, and any
// later one has the key of its rule, so long as UnmarshalPair reads that back as
// the same case, as "" reads back as another.
func (v *accessor) pairKey(accessors []*accessor) (string, error) {
	key, err := v.key()
	present := v.presenceKey()
	if err != nil || present == "" || presenceCase(accessors, present) != v {
		return key, err
	}
	if firstPresence(accessors) == v || pairCase(accessors, present) != nil {
		return key, nil
	}
	return present, nil
}

// firstPresence returns the first case with a "?key" rule.
func firstPresence(accessors []*accessor) *accessor {
	for _, access := range accessors {
		if access.presenceKey() != "" {
			return access
		}
	}
	return nil
}

// MarshalJSON writes the case as its json or txt tag describes. The number or
// boolean of a ",string" case is written as it is, not quoted.
func (v taggedMethods[Storage, Values]) MarshalJSON() ([]byte, error) {
	access := v.tag
	if access == nil {
		if reflect.TypeOf(v.ram) == reflect.TypeOf(json.RawMessage{}) {
			return any(v.ram).(json.RawMessage), nil
		}
		// Data that no case decoded is kept in storage, as it is for any.
		if reflect.ValueOf(&v.ram).Elem().IsZero() {
			return []byte("null"), nil
		}
		return json.Marshal(v.ram)
	}
	if access.text != "" || access.zero {
		if access.fmts {
			return json.Marshal(v.formatted())
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
	if access.void && key != "" {
		return json.Marshal(map[string]any{key: nil})
	}
	return json.Marshal(v.tag.get(&v))
}

// marshalMerged marshals a case selected by a discriminator, adding the key to
// the fields of the value. A value that is not a JSON object has no fields to
// add it to, so it sits under "value", the same shape [taggedMethods.TypesJSON]
// describes. The value's own JSON is worked on, rather than a struct embedding
// the value, as not every type can be embedded and a type with a MarshalJSON
// method would have that method promoted over the discriminator. A nil pointer
// to a struct has no fields, so it is the discriminator alone.
func marshalMerged(value any, key, val string) ([]byte, error) {
	discriminator, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	if value == nil || isNilStructPointer(value) {
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

func isNilStructPointer(value any) bool {
	rvalue := reflect.ValueOf(value)
	return rvalue.Kind() == reflect.Pointer && rvalue.IsNil() && rvalue.Type().Elem().Kind() == reflect.Struct
}

// UnmarshalJSON decodes the case that data selects. The cases are tried in the
// order they are declared, and the first that matches data decides the result:
//   - a case with text matches a JSON string of that text, and any other JSON
//     that is not a string is an error;
//   - a case tagged ",null" matches null, ",number" a number, ",object" an
//     object and ",array" an array, and ",string" a string, which a case of a
//     type other than string reads as text, with [fmt.Sscan] for a number;
//   - a named case matches an object with its name as a key, "?key" an object
//     with that key and "?key=val" an object where that key holds val. The
//     kind after the comma is checked against data, not the value under the
//     name, so a named case of another kind than ",object" never matches here.
//
// A match whose value does not decode is an error. Data that no case matches is
// kept as unknown data, and invalid JSON is an error.
//
// Only when that gives an error, or unknown data, is data tried against the
// cases by its shape, see [taggedMethods.decodeByShape], which reads the forms
// that the order of the cases hides, such as a named case declared after an
// object case, a value under "value" beside a discriminator, a number of a
// ",string" case, or the text of a case with a format. The case found that way
// is only taken if it marshals back to the same JSON as data, so that nothing
// in data is lost. Otherwise the error, or the unknown data, stands.
//
// v is only written once the whole of a case has decoded, so that a failure
// leaves it as it was, and every outcome replaces both its tag and its storage,
// because a tag left over the storage of another case makes a later Get panic on
// JSON that any client sends.
func (v *taggedMethods[Storage, Values]) UnmarshalJSON(data []byte) error {
	var inOrder taggedMethods[Storage, Values]
	err := inOrder.decodeInOrder(data)
	if err == nil && inOrder.tag != nil {
		*v = inOrder
		return nil
	}
	if err == nil && !json.Valid(data) {
		// Kept as unknown data, it would be written back as invalid JSON.
		return json.Unmarshal(data, new(json.RawMessage))
	}
	// Only a case that writes data back as it came is taken, as anything else
	// would lose some of it, where unknown data keeps it all.
	var byShape taggedMethods[Storage, Values]
	if byShape.decodeByShape(data) == nil && byShape.tag != nil && byShape.writes(data) {
		*v = byShape
		return nil
	}
	if err != nil {
		return err
	}
	*v = inOrder
	return nil
}

// writes reports whether v marshals to the same JSON value as data.
func (v taggedMethods[Storage, Values]) writes(data []byte) bool {
	body, err := v.MarshalJSON()
	if err != nil {
		return false
	}
	want, err := decodeExactly(data)
	if err != nil {
		return false
	}
	got, err := decodeExactly(body)
	return err == nil && reflect.DeepEqual(got, want)
}

// decodeInOrder decodes data into the first case, in the order they are declared,
// that it matches, see [taggedMethods.UnmarshalJSON]. It is called on a zero value.
func (v *taggedMethods[Storage, Values]) decodeInOrder(data []byte) error {
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
			// Once a ",string" case of another type is reached, every case after
			// it reads its value as text too.
			if !isString {
				unmarshal = v.unmarshalText
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
		return xray.New(decode(accessors[i], data))
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

// unmarshalText decodes the value of a ",string" case of a type other than
// string, which [taggedMethods.decodeInOrder] reads from a JSON string.
func (v *taggedMethods[Storage, Values]) unmarshalText(access *accessor, data []byte) error {
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

// decodeByShape decodes the case that data selects, so that a value decodes back
// to its own case whatever the order they are declared in. The text of a case
// comes first, followed by the cases that a key of the object selects, followed
// by the cases that match by JSON type, followed by a ",string" case of a number
// or boolean that is written as it is. Within those the first that decodes
// without an error wins, and if none does the first error is returned. It is
// called on a zero value, which has no case afterwards if no case matched.
func (v *taggedMethods[Storage, Values]) decodeByShape(data []byte) error {
	if !json.Valid(data) {
		return json.Unmarshal(data, new(json.RawMessage))
	}
	data = bytes.TrimSpace(data)
	if data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		if matched, err := v.setText(text); matched {
			return err
		}
	}
	var object map[string]json.RawMessage
	if data[0] == '{' {
		if err := json.Unmarshal(data, &object); err != nil {
			return err
		}
	}
	var failed error
	for _, access := range keyedFirst(v.accessors()) {
		for _, value := range access.candidates(data, object) {
			err := v.decodeCase(access, value)
			if err == nil {
				return nil
			}
			if failed == nil {
				failed = err
			}
		}
	}
	// MarshalJSON writes the number or boolean of a ",string" case as it is, so
	// that is its value when no case of that JSON type has taken it.
	for _, access := range v.accessors() {
		if access.quoted() && !access.keyed() && isBareScalar(data) && v.decodeCase(access, data) == nil {
			return nil
		}
	}
	return xray.New(failed)
}

// isBareScalar reports whether data is a JSON number or boolean.
func isBareScalar(data []byte) bool {
	return data[0] == '-' || (data[0] >= '0' && data[0] <= '9') || data[0] == 't' || data[0] == 'f'
}

// keyedFirst puts the cases that a key selects ahead of those that match by JSON
// type. An object case decodes any object, ignoring the fields it does not know,
// so it would otherwise take the value of a named case declared after it.
func keyedFirst(accessors []*accessor) []*accessor {
	ordered := make([]*accessor, 0, len(accessors))
	for _, access := range accessors {
		if access.keyed() {
			ordered = append(ordered, access)
		}
	}
	for _, access := range accessors {
		if !access.keyed() {
			ordered = append(ordered, access)
		}
	}
	return ordered
}

// keyed reports whether a key of the object selects this case.
func (v *accessor) keyed() bool {
	base, _, _ := strings.Cut(v.json, ",")
	return base != ""
}

// candidates returns the JSON in data that could be the value of this case, in
// the order to try it, and nothing if data does not select this case. A case with
// a value that fails to decode leaves the next case to try. object holds the
// fields of data, if it is an object.
func (v *accessor) candidates(data []byte, object map[string]json.RawMessage) [][]byte {
	if v.text != "" || v.zero || v.json == "" {
		return nil
	}
	base, kind, _ := strings.Cut(v.json, ",")
	if kind == "null" {
		if string(data) == "null" {
			return [][]byte{data}
		}
		return nil
	}
	name, rule, _ := strings.Cut(base, "?")
	key, val, hasConst := strings.Cut(rule, "=")
	switch {
	case rule != "" && !ruleHolds(object, key, val, hasConst):
		return nil
	case rule != "" && name != "":
		return [][]byte{object[name]}
	case hasConst:
		return v.mergedCandidates(data, object, key)
	case rule != "":
		return [][]byte{data}
	}
	if name == "" {
		if v.matchesKind(kind, data, false) {
			return [][]byte{data}
		}
		return nil
	}
	value, ok := object[name]
	if ok && (string(value) == "null" || v.matchesKind(kind, value, true)) {
		return [][]byte{value}
	}
	return nil
}

// ruleHolds reports whether a rule picks a case: "?key" when the object has the
// key, "?key=val" when the key holds the string val.
func ruleHolds(object map[string]json.RawMessage, key, val string, hasConst bool) bool {
	value, exists := object[key]
	if !exists {
		return false
	}
	if !hasConst {
		return true
	}
	var s string
	return json.Unmarshal(value, &s) == nil && s == val
}

// matchesKind reports whether a value is of the JSON type that a case is tagged
// with. A number under the key of a named ",string" case is left to decode, as
// that is how it is marshalled.
func (v *accessor) matchesKind(kind string, value []byte, named bool) bool {
	switch kind {
	case "string":
		return value[0] == '"' || (named && !v.void && v.rtyp.Kind() != reflect.String)
	case "number":
		return value[0] == '-' || (value[0] >= '0' && value[0] <= '9')
	case "object":
		return value[0] == '{'
	case "array":
		return value[0] == '['
	}
	return true
}

// mergedCandidates returns where the value of a case can be in an object that
// was selected by a discriminator, see [marshalMerged]. Only an object with no
// other field can be a value wrapped under "value", and that is tried first
// unless the value is a struct: a pointer, interface or map decodes from any
// object, and the fields of a struct could be just that. The discriminator is
// not a part of a value that keeps every field it is given.
func (v *accessor) mergedCandidates(data []byte, object map[string]json.RawMessage, key string) [][]byte {
	if v.void {
		return [][]byte{data}
	}
	whole := data
	if v.rtyp.Kind() == reflect.Map || v.rtyp.Kind() == reflect.Interface {
		whole = withoutField(data, object, key)
	}
	value, wrapped := object["value"]
	if !wrapped {
		return [][]byte{whole}
	}
	if len(object) == 2 && v.rtyp.Kind() != reflect.Struct {
		return [][]byte{value, whole}
	}
	return [][]byte{whole, value}
}

// withoutField returns the object in data without the given field. The fields
// were decoded from valid JSON, so they will always encode.
func withoutField(data []byte, object map[string]json.RawMessage, key string) []byte {
	fields := maps.Clone(object)
	delete(fields, key)
	body, err := json.Marshal(fields)
	if err != nil {
		return data
	}
	return body
}

// quoted reports whether a case is a type that a ",string" tag changes the JSON
// of, the same as in encoding/json.
func (v *accessor) quoted() bool {
	_, kind, _ := strings.Cut(v.json, ",")
	if kind != "string" || v.rtyp == nil {
		return false
	}
	switch v.rtyp.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

// unmarshal decodes the JSON value of this case into ptr. Most values of a
// ",string" case are JSON strings of their own, such as a time or a []byte.
func (v *accessor) unmarshal(value []byte, ptr any) error {
	err := json.Unmarshal(value, ptr)
	if err == nil || !v.quoted() {
		return err
	}
	var text string
	if json.Unmarshal(value, &text) != nil || text == "null" {
		return err
	}
	return json.Unmarshal([]byte(text), ptr)
}

func (v *taggedMethods[Storage, Values]) decodeCase(access *accessor, value []byte) error {
	if access.void {
		return v.setCase(access)
	}
	ptr := reflect.New(access.rtyp)
	if err := access.unmarshal(value, ptr.Interface()); err != nil {
		return err
	}
	return v.setDecoded(access, ptr.Elem().Interface())
}

// setText makes v the case with the given text, reporting whether there is one.
// Exact text goes first, as a format declared ahead of it could match it too.
func (v *taggedMethods[Storage, Values]) setText(text string) (bool, error) {
	accessors := v.accessors()
	for _, access := range accessors {
		if access.hasExactText(text) {
			return true, v.setCase(access)
		}
	}
	for _, access := range accessors {
		if val, ok := access.scan(text); ok {
			return true, v.setDecoded(access, val)
		}
	}
	return false, nil
}

// formatted returns the text of a case with a format. String storage holds it
// already, and a value cannot always be scanned back out of that to format it.
func (v taggedMethods[Storage, Values]) formatted() string {
	if ram := reflect.ValueOf(&v.ram).Elem(); ram.Kind() == reflect.String {
		return ram.String()
	}
	return fmt.Sprintf(v.tag.text, expand(v.tag.get(&v))...)
}

// setCase makes v the given case with nothing in storage, which is also
// what the accessor of a case without a value holds.
func (v *taggedMethods[Storage, Values]) setCase(access *accessor) error {
	next := taggedMethods[Storage, Values]{tag: access}
	// String storage holds the text of a case with a value, see [accessor.as],
	// and a decoded value has to equal one that was made with As.
	if ram := reflect.ValueOf(&next.ram).Elem(); ram.Kind() == reflect.String && !access.void {
		ram.SetString(access.text)
	}
	*v = next
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
			return []byte(v.formatted()), nil
		}
		return []byte(access.text), nil
	}
	return nil, errors.New("cannot marshal non-text variant")
}

// UnmarshalText decodes the case with the given text: the first case, in the
// order they are declared, whose txt tag is the text, even one with a format,
// or else a case with a format that writes the text for a value. Empty text that
// no case has is the zero value, which is what [taggedMethods.MarshalText] gives
// for it. Text that no case has leaves v as it was, without an error.
func (v *taggedMethods[Storage, Values]) UnmarshalText(data []byte) error {
	text := string(data)
	accessors := v.accessors()
	for _, access := range accessors {
		if (access.text != "" || access.zero) && access.text == text {
			return v.setCase(access)
		}
	}
	for _, access := range accessors {
		if val, ok := access.scan(text); ok {
			return v.setDecoded(access, val)
		}
	}
	if len(data) == 0 {
		*v = taggedMethods[Storage, Values]{}
	}
	return nil
}

// Validate is here for [Validator], and reports false for every value: it does
// not check val against the cases.
func (taggedMethods[Storage, Values]) Validate(val any) bool {
	return false
}

func (v taggedMethods[Storage, Values]) append(impl *accessor) {
	// An added case must not be mistaken for the cases of the type, which is
	// what cache holds once it has an entry.
	v.accessors()
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
	position := 0
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		if field.Name == "_" {
			continue
		}
		access := accessors[position]
		position++
		if !access.safe {
			panic(fmt.Sprintf("unsafe use of variant accessor '%s': incompatible with storage", field.Name))
		}
		type settable interface {
			set(*accessor)
		}
		rvalue.Field(i).Addr().Interface().(settable).set(access)
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
		if val, ok := v.scan(rvalue.String()); ok {
			return val
		}
		return reflect.Zero(v.rtyp).Interface()
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
	if storage.Kind() == reflect.String {
		return true
	}
	if storage.Kind() != reflect.Interface {
		return reflect.TypeOf(val) == v.rtyp
	}
	return val == nil || reflect.TypeOf(val).AssignableTo(storage)
}

// hasExactText reports whether text is the text of a case with no format.
func (v *accessor) hasExactText(text string) bool {
	return (v.text != "" || v.zero) && !v.fmts && v.text == text
}

// scan reads the value of a case out of its text, the reverse of formatting it
// with the case's format. Scanning is looser than formatting, so text is only
// accepted if formatting the value gives the same text back. Text cannot show
// where a value ends if it contains what follows it in the format, so a string
// holding that comes back split at the first place it appears.
func (v *accessor) scan(text string) (any, bool) {
	if !v.fmts || v.rtyp == nil {
		return nil, false
	}
	value := reflect.New(v.rtyp).Elem()
	if !scanFormat(v.text, text, tupleFields(value, nil)) {
		return nil, false
	}
	val := value.Interface()
	return val, fmt.Sprintf(v.text, expand(val)...) == text
}

// scanFormat sets each target from the text that lines up with its verb. Each
// value has its own text, as fmt would stop a string at a space.
func scanFormat(format, text string, targets []reflect.Value) bool {
	for {
		literal, verb, rest := nextVerb(format)
		if !strings.HasPrefix(text, literal) {
			return false
		}
		text = text[len(literal):]
		if verb == "" {
			return text == "" && len(targets) == 0
		}
		if len(targets) == 0 {
			return false
		}
		end := len(text)
		if following, _, _ := nextVerb(rest); following != "" {
			end = strings.Index(text, following)
			if end < 0 {
				return false
			}
		}
		if !scanVerb(verb, text[:end], targets[0]) {
			return false
		}
		format, text, targets = rest, text[end:], targets[1:]
	}
}

// nextVerb splits a format at its next verb, returning the text before it and
// the rest of the format. The flags, width and precision of the verb are left out:
// fmt cannot scan with a precision, and the text of each verb is already known.
func nextVerb(format string) (literal, verb, rest string) {
	var text strings.Builder
	for {
		before, after, found := strings.Cut(format, "%")
		text.WriteString(before)
		if !found {
			return text.String(), "", ""
		}
		if strings.HasPrefix(after, "%") {
			text.WriteByte('%')
			format = after[1:]
			continue
		}
		start := strings.IndexFunc(after, isVerb)
		if start < 0 {
			return text.String(), "", ""
		}
		_, size := utf8.DecodeRuneInString(after[start:])
		return text.String(), "%" + after[start:start+size], after[start+size:]
	}
}

func isVerb(r rune) bool { return !strings.ContainsRune("+-# 0123456789.", r) }

func scanVerb(verb, text string, into reflect.Value) bool {
	if verb == "%v" || verb == "%s" {
		switch {
		case into.Kind() == reflect.String:
			into.SetString(text)
			return true
		case into.Kind() == reflect.Slice && into.Type().Elem().Kind() == reflect.Uint8:
			into.SetBytes([]byte(text))
			return true
		case into.Kind() == reflect.Interface && into.NumMethod() == 0:
			into.Set(reflect.ValueOf(text))
			return true
		}
		if scanner, ok := into.Addr().Interface().(encoding.TextUnmarshaler); ok {
			return scanner.UnmarshalText([]byte(text)) == nil
		}
	}
	scanned, err := fmt.Sscanf(text, verb, into.Addr().Interface())
	return err == nil && scanned == 1
}

// tupleFields appends value to into, or each value in it if it is a tuple, in
// the same order that [expand] gives them to a format.
func tupleFields(value reflect.Value, into []reflect.Value) []reflect.Value {
	if _, isTuple := value.Interface().(expander); !isTuple {
		return append(into, value)
	}
	for i := range value.NumField() {
		into = tupleFields(value.Field(i), into)
	}
	return into
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
	if rvalue.Kind() != reflect.Interface && rvalue.Kind() != reflect.String && reflect.TypeOf(val) != v.rtyp {
		panic("unsafe use of variant accessor")
	}
	switch rvalue.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
		reflect.Array, reflect.Struct:
		// [accessor.get] reads the storage as the type of the case, whatever the
		// type of the storage is, so its bytes have to be the value's own.
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
	var parent Variant
	if typed, ok := any(parent).(interface{ accessors() []*accessor }); ok {
		return v.accessor.pairKey(typed.accessors())
	}
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
		key, err := access.pairKey(accessors)
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
	var types []reflect.Type
	for _, access := range v.described() {
		types = append(types, access.typeJSON())
	}
	return types
}

// described returns the cases that the type declares. Another type can add a
// case to it, which decodes but belongs to that type, unless this is an [Extern]
// where the added cases are the only ones.
func (v taggedMethods[Storage, Values]) described() []*accessor {
	declared := 0
	var rtype = reflect.TypeOf([0]Values{}).Elem()
	for i := 0; i < rtype.NumField(); i++ {
		if rtype.Field(i).Name != "_" {
			declared++
		}
	}
	accessors := v.accessors()
	if declared == 0 {
		return accessors
	}
	return accessors[:declared]
}

// typeJSON returns a type with the same JSON shape as a value of this case.
func (v *accessor) typeJSON() reflect.Type {
	if v.text != "" || v.zero {
		return reflect.TypeOf("")
	}
	base, kind, _ := strings.Cut(v.json, ",")
	name, rule, _ := strings.Cut(base, "?")
	key, _, hasConst := strings.Cut(rule, "=")
	valueType := v.rtyp
	if valueType == nil {
		valueType = reflect.TypeOf(struct{}{})
	}
	if name != "" {
		// Wrapped as {"name": value} with optional discriminator. The fields
		// keep the names they had when a discriminator merged the value, so the
		// type is the one it was where its name was "value".
		valueField, typeField := "Value", "Type"
		if hasConst {
			valueField, typeField = "UnionValue", "UnionType"
		}
		fields := []reflect.StructField{
			{
				Name: valueField,
				Type: valueType,
				Tag:  reflect.StructTag(`json:"` + name + `"`),
			},
		}
		if key != "" {
			fields = append(fields, reflect.StructField{
				Name: typeField,
				Type: reflect.TypeOf(""),
				Tag:  reflect.StructTag(`json:"` + key + `"`),
			})
		}
		return reflect.StructOf(fields)
	}
	if hasConst {
		// Discriminated union: value fields merged with {key: val}.
		return mergedType(valueType, key)
	}
	if v.void && key != "" {
		return reflect.StructOf([]reflect.StructField{{
			Name: "Value",
			Type: reflect.TypeFor[any](),
			Tag:  reflect.StructTag(`json:"` + key + `"`),
		}})
	}
	// Bare value — use the kind hint or the value type directly. A case without
	// a value is described as an array of empty objects, any of which it reads.
	switch {
	case kind == "string":
		return reflect.TypeOf("")
	case kind == "number":
		return reflect.TypeOf(float64(0))
	case kind == "array" && v.void:
		return reflect.SliceOf(valueType)
	}
	return valueType
}

// mergedType returns the shape that [marshalMerged] gives a value of a type. It
// can only tell from the type, so a type that writes its own JSON is taken to
// write something other than an object.
func mergedType(value reflect.Type, key string) reflect.Type {
	discriminator := reflect.StructField{
		Name: "UnionType",
		Type: reflect.TypeOf(""),
		Tag:  reflect.StructTag(`json:"` + key + `"`),
	}
	target := value
	if target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	if target.Kind() == reflect.Map {
		return reflect.TypeFor[map[string]any]()
	}
	if target.Kind() != reflect.Struct || writesOwnJSON(value) {
		return reflect.StructOf([]reflect.StructField{{
			Name: "UnionValue",
			Type: value,
			Tag:  reflect.StructTag(`json:"value"`),
		}, discriminator})
	}
	// Fields of an embedded struct are copied rather than embedded, because
	// [reflect.StructOf] cannot embed a type with methods. A field with the
	// name of the discriminator is replaced by it, see [marshalMerged].
	var fields []reflect.StructField
	for _, field := range reflect.VisibleFields(target) {
		if !field.IsExported() || isEmbeddedStruct(field) || jsonName(field) == key {
			continue
		}
		fields = append(fields, reflect.StructField{Name: field.Name, Type: field.Type, Tag: field.Tag})
	}
	return reflect.StructOf(append(fields, discriminator))
}

// jsonName returns the name that encoding/json gives a field.
func jsonName(field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	if name == "" {
		return field.Name
	}
	return name
}

func writesOwnJSON(rtype reflect.Type) bool {
	return rtype.Implements(reflect.TypeFor[json.Marshaler]()) || rtype.Implements(reflect.TypeFor[encoding.TextMarshaler]())
}

func isEmbeddedStruct(field reflect.StructField) bool {
	if field.Type.Kind() == reflect.Pointer {
		return field.Anonymous && field.Type.Elem().Kind() == reflect.Struct
	}
	return field.Anonymous && field.Type.Kind() == reflect.Struct
}

func (v taggedMethods[Storage, Values]) Reflection() []CaseReflection {
	var cases []CaseReflection
	for _, access := range v.described() {
		cases = append(cases, CaseReflection{
			Name: access.name,
			Tags: access.rtag,
			Vary: access.rtyp,
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
