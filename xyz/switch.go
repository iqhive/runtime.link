package xyz

import (
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
	bool | int | int8 | int16 | int32 | int64 | uint8 | uint16 | uint32 | uint64 | uint | uintptr | string | float32 | float64 | complex64 | complex128
}

// switchMethods is embedded into Switch to ensure
// any switch values have the following methods.
type switchMethods[Storage any, Values any] struct {
	ram Storage
}

// Raw returns the underlying storage value.
func (v switchMethods[Storage, Values]) Raw() Storage { return v.ram }

// String implements [fmt.Stringer].
func (v switchMethods[Storage, Values]) String() string {
	rtype := reflect.TypeOf([0]Values{}).Elem()
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		value := reflect.ValueOf(v.ram)
		switch value.Kind() {
		case reflect.String:
			name, ok := field.Tag.Lookup("json")
			if !ok {
				name = field.Name
			}
			if value.String() == name {
				return field.Name
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if value.Int() == int64(i) {
				return field.Name
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if value.Uint() == uint64(i) {
				return field.Name
			}
		case reflect.Float32, reflect.Float64:
			if value.Float() == float64(i) {
				return field.Name
			}
		case reflect.Complex64, reflect.Complex128:
			if value.Complex() == complex(float64(i), 0) {
				return field.Name
			}
		case reflect.Bool:
			if value.Bool() == (i == 0) {
				return field.Name
			}
		}
	}
	return fmt.Sprint(v.ram)
}

// MarshalJSON implements [json.Marshaler].
func (v switchMethods[Storage, Values]) MarshalJSON() ([]byte, error) {
	rtype := reflect.TypeOf([0]Values{}).Elem()
	hjson := false
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		name, ok := field.Tag.Lookup("json")
		if !ok {
			name = field.Name
		} else {
			hjson = true
		}
		value := reflect.ValueOf(v.ram)
		switch value.Kind() {
		case reflect.String:
			if value.String() == name {
				return json.Marshal(name)
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if value.Int() == int64(i) {
				return json.Marshal(name)
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if value.Uint() == uint64(i) {
				return json.Marshal(name)
			}
		case reflect.Float32, reflect.Float64:
			if value.Float() == float64(i) {
				return json.Marshal(name)
			}
		case reflect.Complex64, reflect.Complex128:
			if value.Complex() == complex(float64(i), 0) {
				return json.Marshal(name)
			}
		case reflect.Bool:
			if value.Bool() == (i != 0) {
				return json.Marshal(name)
			}
		}
	}
	if hjson {
		return json.Marshal(fmt.Sprint(v.ram))
	}
	return json.Marshal(v.ram)
}

// UnmarshalJSON implements [json.Unmarshaler].
func (v *switchMethods[Storage, Values]) UnmarshalJSON(data []byte) error {
	if data[0] == '"' && reflect.TypeOf(v.ram).Kind() != reflect.String {
		rtype := reflect.TypeOf([0]Values{}).Elem()
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		for i := 0; i < rtype.NumField(); i++ {
			field := rtype.Field(i)
			name, ok := field.Tag.Lookup("json")
			if !ok {
				name = field.Name
			}
			if s == name {
				switch ptr := any(&v.ram).(type) {
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
				case *float32:
					*ptr = float32(i)
				case *float64:
					*ptr = float64(i)
				case *complex64:
					*ptr = complex(float32(i), 0)
				case *complex128:
					*ptr = complex(float64(i), 0)
				}
				return nil
			}
		}
	}
	return json.Unmarshal(data, &v.ram)
}

// MarshalText implements [encoding.TextMarshaler].
func (v switchMethods[Storage, Values]) MarshalText() ([]byte, error) {
	rtype := reflect.TypeOf([0]Values{}).Elem()
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		name, ok := field.Tag.Lookup("json")
		if !ok {
			name = field.Name
		}
		value := reflect.ValueOf(v.ram)
		switch value.Kind() {
		case reflect.String:
			if value.String() == name {
				return []byte(name), nil
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if value.Int() == int64(i) {
				return []byte(name), nil
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
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
	return []byte(fmt.Sprint(v.ram)), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler].
func (v *switchMethods[Storage, Values]) UnmarshalText(data []byte) error {
	if reflect.TypeOf(v.ram).Kind() != reflect.String {
		rtype := reflect.TypeOf([0]Values{}).Elem()
		for i := 0; i < rtype.NumField(); i++ {
			field := rtype.Field(i)
			name, ok := field.Tag.Lookup("json")
			if !ok {
				name = field.Name
			}
			if string(data) == name {
				switch ptr := any(&v.ram).(type) {
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
				case *float32:
					*ptr = float32(i)
				case *float64:
					*ptr = float64(i)
				case *complex64:
					*ptr = complex(float32(i), 0)
				case *complex128:
					*ptr = complex(float64(i), 0)
				}
				return nil
			}
		}
	}
	_, err := fmt.Sscan(string(data), &v.ram)
	return err
}

func (v *switchMethods[Storage, Values]) pointer() any { return &v.ram }

func (v switchMethods[Storage, Values]) Values(internal) Values {
	type pointable interface{ pointer() any }
	var values Values
	var rvalue = reflect.ValueOf(&values).Elem()
	var rtype = reflect.TypeOf(values)
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		switch ptr := rvalue.Field(i).Addr().Interface().(pointable).pointer().(type) {
		case *string:
			name, ok := field.Tag.Lookup("json")
			if !ok {
				name = field.Name
			}
			*ptr = name
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
	return values
}

func (v switchMethods[Storage, Values]) ValuesJSON() (oneof []json.RawMessage) {
	type pointable interface{ pointer() any }
	var values Values
	var rtype = reflect.TypeOf(values)
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		name, ok := field.Tag.Lookup("json")
		if !ok {
			name = field.Name
		}
		b, err := json.Marshal(name)
		if err != nil {
			continue
		}
		oneof = append(oneof, b)
	}
	return oneof
}
