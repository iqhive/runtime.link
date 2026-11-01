package xyz

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"reflect"
)

// Switch on the underlying Storage, which can be any one of the
// [Values].
type Switch[Storage switchable, Values any] struct {
	switchMethods[Storage, Values] // export methods.
}

type switchWith[Storage any, Values any] interface {
	~struct {
		switchMethods[Storage, Values]
	}
	Values(internal) Values
}

// Raw returns a switch value of the given type, with the given storage.
func Raw[T switchWith[Storage, Values], Storage any, Values any](val Storage) T {
	var zero T
	raw := (struct {
		switchMethods[Storage, Values]
	})(zero)
	raw.ram = val
	return T(raw)
}

type switchable interface {
	bool | int | int8 | int16 | int32 | int64 | uint8 | uint16 | uint32 | uint64 | uint | uintptr | string | float32 | float64
}

// switchMethods is embedded into Switch to ensure
// any switch values have the following methods.
type switchMethods[Storage any, Values any] struct {
	ram Storage
}

// Raw returns the underlying storage value.
func (v switchMethods[Storage, Values]) Raw() Storage          { return v.ram }
func (v *switchMethods[Storage, Values]) RawPointer() *Storage { return &v.ram }

// String implements [fmt.Stringer].
func (v switchMethods[Storage, Values]) String() string {
	b, err := v.MarshalText()
	if err != nil {
		return fmt.Sprint(v.ram)
	}
	return string(b)
}

func (v switchMethods[Storage, Values]) Interface() any      { return v.ram }
func (v *switchMethods[Storage, Values]) InterfaceAddr() any { return &v.ram }

func (v switchMethods[Storage, Values]) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(v.ram); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (v *switchMethods[Storage, Values]) GobDecode(data []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(data))
	return dec.Decode(&v.ram)
}

// UnmarshalJSON implements [json.Unmarshaler]. A JSON string is the name of a
// case, or for numeric storage, a value outside the set that [marshalEnumText]
// wrote as a number in a string.
func (v *switchMethods[Storage, Values]) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' && reflect.TypeOf(v.ram).Kind() != reflect.String {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		if unmarshalEnumName(reflect.TypeFor[Values](), any(&v.ram), text) {
			return nil
		}
		err := json.Unmarshal(data, &v.ram)
		if err == nil {
			return nil
		}
		value, ok := scanEnumText[Storage](text)
		if !ok {
			return err
		}
		v.ram = value
		return nil
	}
	return json.Unmarshal(data, &v.ram)
}

// scanEnumText reads a value that [marshalEnumText] wrote with fmt, and nothing
// else: the text has to be what formatting the value gives back.
func scanEnumText[Storage any](text string) (Storage, bool) {
	var value Storage
	if _, err := fmt.Sscan(text, &value); err != nil {
		return value, false
	}
	return value, fmt.Sprint(value) == text
}

// MarshalText implements [encoding.TextMarshaler].
func (v switchMethods[Storage, Values]) MarshalText() ([]byte, error) {
	return marshalEnumText(reflect.TypeFor[Values](), reflect.ValueOf(v.ram))
}

// UnmarshalText implements [encoding.TextUnmarshaler]. String storage holds the
// text whole. Other storage takes the name of a case, or a value read with
// [fmt.Sscan], which reads as much of the text as it can and ignores the rest.
// Empty text leaves other storage as it was.
func (v *switchMethods[Storage, Values]) UnmarshalText(data []byte) error {
	// fmt.Sscan would cut the text at a space, and leave it alone if empty.
	if text, ok := any(&v.ram).(*string); ok {
		*text = string(data)
		return nil
	}
	if unmarshalEnumName(reflect.TypeFor[Values](), any(&v.ram), string(data)) {
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	_, err := fmt.Sscan(string(data), &v.ram)
	return err
}

func (v *switchMethods[Storage, Values]) pointer() any { return &v.ram }

func (v switchMethods[Storage, Values]) Values(internal) Values {
	var values Values
	populateEnumValues(reflect.ValueOf(&values).Elem())
	return values
}

func (v switchMethods[Storage, Values]) ValuesJSON() (oneof []json.RawMessage) {
	return enumValuesJSON(reflect.TypeFor[Values]())
}

// enumFieldName returns the name of a case of a switch: its whole json tag, any
// options included, or the name of the field if it has no json tag.
func enumFieldName(field reflect.StructField) string {
	name, ok := field.Tag.Lookup("json")
	if !ok {
		name = field.Name
	}
	return name
}

// setEnumOrdinal writes the ordinal i into the storage pointed to by ramPtr,
// matching the numeric kind of the underlying storage. It is intentionally
// non-generic so it is compiled once rather than stenciled per switch type.
func setEnumOrdinal(ramPtr any, i int) {
	switch ptr := ramPtr.(type) {
	case *bool:
		*ptr = i != 0
	case *int:
		*ptr = i
	case *int8:
		*ptr = int8(i)
	case *int16:
		*ptr = int16(i)
	case *int32:
		*ptr = int32(i)
	case *int64:
		*ptr = int64(i)
	case *uint:
		*ptr = uint(i)
	case *uint8:
		*ptr = uint8(i)
	case *uint16:
		*ptr = uint16(i)
	case *uint32:
		*ptr = uint32(i)
	case *uint64:
		*ptr = uint64(i)
	case *uintptr:
		*ptr = uintptr(i)
	case *float32:
		*ptr = float32(i)
	case *float64:
		*ptr = float64(i)
	case *complex64:
		*ptr = complex(float32(i), 0)
	case *complex128:
		*ptr = complex(float64(i), 0)
	}
}

// unmarshalEnumName matches the literal name against the switch's fields and,
// on a match, writes that field's ordinal into ramPtr.
func unmarshalEnumName(rtype reflect.Type, ramPtr any, name string) bool {
	for i := 0; i < rtype.NumField(); i++ {
		if name == enumFieldName(rtype.Field(i)) {
			setEnumOrdinal(ramPtr, i)
			return true
		}
	}
	return false
}

// marshalEnumText renders the enum value as the name of its matching field.
func marshalEnumText(rtype reflect.Type, value reflect.Value) ([]byte, error) {
	for i := 0; i < rtype.NumField(); i++ {
		name := enumFieldName(rtype.Field(i))
		switch value.Kind() {
		case reflect.String:
			if value.String() == name {
				return []byte(name), nil
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if value.Int() == int64(i) {
				return []byte(name), nil
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			if value.Uint() == uint64(i) {
				return []byte(name), nil
			}
		case reflect.Float32, reflect.Float64:
			if value.Float() == float64(i) {
				return []byte(name), nil
			}
		case reflect.Complex64, reflect.Complex128:
			if value.Complex() == complex(float64(i), 0) {
				return []byte(name), nil
			}
		case reflect.Bool:
			if value.Bool() == (i != 0) {
				return []byte(name), nil
			}
		}
	}
	return fmt.Append(nil, value.Interface()), nil
}

// populateEnumValues fills each field of the accessor struct (addressed by
// rvalue) with the canonical value for that case.
func populateEnumValues(rvalue reflect.Value) {
	type pointable interface{ pointer() any }
	rtype := rvalue.Type()
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		switch ptr := rvalue.Field(i).Addr().Interface().(pointable).pointer().(type) {
		case *string:
			*ptr = enumFieldName(field)
		default:
			setEnumOrdinal(ptr, i)
		}
	}
}

// enumValuesJSON returns the JSON-encoded field names of the switch type.
func enumValuesJSON(rtype reflect.Type) (oneof []json.RawMessage) {
	for i := 0; i < rtype.NumField(); i++ {
		b, err := json.Marshal(enumFieldName(rtype.Field(i)))
		if err != nil {
			continue
		}
		oneof = append(oneof, b)
	}
	return oneof
}
