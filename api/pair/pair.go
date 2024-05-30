// Package pair can represent 1:1 mappings between two different APIs, such that validation errors are transformed.
package pair

import (
	"reflect"
)

// Registry of functions that convert errors into [Error]s.
type Registry struct {
	functions []func(error) Error
}

// Register a function that returns an [Error] from an error,
// if the function returns nil, the error will not be converted.
func (registry *Registry) Register(fn func(error) Error) {
	registry.functions = append(registry.functions, fn)
}

// Make a 1:1 mapping function that maps from -> into and into -> from, as well as an
// error wrapper that transforms field-specific errors from one API space into the
// other.
func Make[From, Into any](registry Registry, mapping func(from *From, into *Into) []Field) (func(From) Into, func(Into) From, func(error) error) {
	toInto := func(from From) (into Into) {
		for _, field := range mapping(&from, &into) {
			field.toInto()
		}
		return
	}
	toFrom := func(into Into) (from From) {
		for _, field := range mapping(&from, &into) {
			field.toFrom()
		}
		return
	}
	wrap := func(err error) error {
		if err == nil {
			return nil
		}
		hasField, ok := err.(Error)
		if !ok {
			for _, fn := range registry.functions {
				if converted := fn(err); converted != nil {
					hasField = converted
				}
			}
			if hasField == nil {
				return err
			}
		}

		lookingFor := hasField.FieldName()

		var from = reflect.ValueOf(new(From)).Elem()
		var into = reflect.ValueOf(new(Into)).Elem()

		fields := mapping(from.Addr().Interface().(*From), into.Addr().Interface().(*Into))

		for i := 0; i < into.Type().NumField(); i++ {
			field := into.Type().Field(i)
			if field.Name == lookingFor {
				for _, field := range fields {
					if field.isInto == into.Field(i).Addr().Interface() {
						for i := 0; i < from.Type().NumField(); i++ {
							fromField := from.Type().Field(i)
							if field.isFrom == from.Field(i).Addr().Interface() {
								return hasField.WithFieldNameSetTo(fromField.Name)
							}
						}
					}
				}
			}
		}
		return err
	}
	return toInto, toFrom, wrap
}

// Error that relates to a particular field from one API model.
type Error interface {
	FieldName() string
	WithFieldNameSetTo(string) error
}

// Field represents a 1:1 mapping between two fields.
type Field struct {
	toInto func()
	toFrom func()
	isFrom any
	isInto any
}

// Bool pairs two bool fields together.
func Bool[From ~bool, Into ~bool](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Int8 pairs two int8 fields together.
func Int8[From ~int8, Into ~int8](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Int16 pairs two int16 fields together.
func Int16[From ~int16, Into ~int16](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Int32 pairs two int32 fields together.
func Int32[From ~int32, Into ~int32](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Int64 pairs two int64 fields together.
func Int64[From ~int64, Into ~int64](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Int pairs two int fields together.
func Int[From ~int, Into ~int](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Uint8 pairs two uint8 fields together.
func Uint8[From ~uint8, Into ~uint8](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Uint16 pairs two uint16 fields together.
func Uint16[From ~uint16, Into ~uint16](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Uint32 pairs two uint32 fields together.
func Uint32[From ~uint32, Into ~uint32](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Uint64 pairs two uint64 fields together.
func Uint64[From ~uint64, Into ~uint64](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Uint pairs two uint fields together.
func Uint[From ~uint, Into ~uint](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Uintptr pairs two uintptr fields together.
func Uintptr[From ~uintptr, Into ~uintptr](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Float32 pairs two float32 fields together.
func Float32[From ~float32, Into ~float32](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Float64 pairs two float64 fields together.
func Float64[From ~float64, Into ~float64](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Complex64 pairs two complex64 fields together.
func Complex64[From ~complex64, Into ~complex64](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Complex128 pairs two complex128 fields together.
func Complex128[From ~complex128, Into ~complex128](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Slice pairs two slice fields together.
func Slice[From, Into any](from *[]From, into *[]Into, rule func(*From, *Into) Field) Field {
	return Field{
		toInto: func() {
			*into = make([]Into, len(*from))
			for i, val := range *from {
				rule(&val, &(*into)[i]).toInto()
			}
		},
		toFrom: func() {
			*from = make([]From, len(*into))
			for i, val := range *into {
				rule(&(*from)[i], &val).toFrom()
			}
		},
		isFrom: from,
		isInto: into,
	}
}

// Map pairs two map fields together.
func Map[FromKey, IntoKey comparable, FromVal, IntoVal any](
	from *map[FromKey]FromVal, into *map[IntoKey]IntoVal,
	keyRule func(*FromKey, *IntoKey) Field,
	valRule func(*FromVal, *IntoVal) Field,
) Field {
	return Field{
		toInto: func() {
			*into = make(map[IntoKey]IntoVal)
			for key, val := range *from {
				keyRule(&key, new(IntoKey)).toInto()
				valRule(&val, new(IntoVal)).toInto()
				(*into)[*new(IntoKey)] = *new(IntoVal)
			}
		},
		toFrom: func() {
			*from = make(map[FromKey]FromVal)
			for key, val := range *into {
				keyRule(new(FromKey), &key).toFrom()
				valRule(new(FromVal), &val).toFrom()
				(*from)[*new(FromKey)] = *new(FromVal)
			}
		},
		isFrom: from,
		isInto: into,
	}
}

// String pairs two string fields together.
func String[From ~string, Into ~string](from *From, into *Into) Field {
	return Field{
		toInto: func() { *into = Into(*from) },
		toFrom: func() { *from = From(*into) },
		isFrom: from,
		isInto: into,
	}
}

// Const returns a constant value, that is assumed to always be present
// on one side of an API pairing.
func Const[T any](value T) *T { return &value }
