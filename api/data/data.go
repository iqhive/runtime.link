// Package data provides ways to declare validation constraints on values, these constraints can be reflected upon at runtime.
package data

import (
	"fmt"
	"reflect"
	"strings"
)

// Validator is a value with validation. The zero value for
// a Validator must always return an error when validated.
type Validator interface {
	Validate() error
}

type numeric interface {
	~int | ~uint | ~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64 | ~complex64 | ~complex128
}

type ReportingObject[T any] struct {
	// Reports checks the given errors and returns a new error if any
	// of them are not nil.
	Reports func(...error) error
}

type errReports struct {
	mirror

	rvalue reflect.Value
	Errors []error
}

func (report *errReports) Unwrap() []error { return report.Errors }

func (report *errReports) reflect(field reflect.Value, index []reflect.StructField, info reflect.StructField) {
	if field.Interface() == report.value {
		report.index = index
		report.field = info

		parent := []reflect.StructField{info}
		for _, err := range report.Errors {
			setter, ok := err.(setter)
			if ok {
				for i := 0; i < report.rvalue.NumField(); i++ {
					field := report.rvalue.Field(i)
					setter.reflect(field.Addr(), parent, report.rvalue.Type().Field(i))
				}
			}
		}
	}
}

func (report *errReports) Error() string {
	var message strings.Builder
	for i, err := range report.Errors {
		if i > 0 && i == len(report.Errors)-1 {
			message.WriteString(" and ")
		} else if i > 0 {
			message.WriteByte(',')
			message.WriteByte(' ')
		}
		message.WriteString(err.Error())
	}
	return message.String()
}

func Object[T any](value *T) ReportingObject[T] {
	rvalue := reflect.ValueOf(value)
	for rvalue.Kind() == reflect.Ptr {
		rvalue = rvalue.Elem()
	}
	return ReportingObject[T]{
		Reports: func(errs ...error) error {
			var group []error
			for _, err := range errs {
				if err != nil {
					if rvalue.Kind() == reflect.Struct {
						setter, ok := err.(setter)
						if ok {
							for i := 0; i < rvalue.NumField(); i++ {
								field := rvalue.Field(i)
								setter.reflect(field.Addr(), nil, rvalue.Type().Field(i))
							}
						}
					}
					group = append(group, err)
				}
			}
			if len(group) == 0 {
				return nil
			}
			return &errReports{
				mirror: mirror{value: value},
				Errors: group,
			}
		},
	}
}

type setter interface {
	reflect(reflect.Value, []reflect.StructField, reflect.StructField)
}

// Exists runs the given function on a pointer value only if
// the value is not nil.
func Exists[T any](value **T, fn func(*T) error) error {
	if *value == nil {
		return nil
	}
	return fn(*value)
}

// Absent reports an error if the pointer is nil.
func Absent[T any](value **T) error {
	if *value == nil {
		return &ErrMissing{
			mirror: mirror{value: value},
		}
	}
	return nil
}

type mirror struct {
	value any
	index []reflect.StructField
	field reflect.StructField
}

func (err *mirror) StatusHTTP() int { return 400 }

func (err *mirror) reflect(field reflect.Value, index []reflect.StructField, info reflect.StructField) {
	if field.Interface() == err.value {
		err.index = index
		err.field = info
	}
}

func (err *mirror) FieldName() string {
	if err.field.Name == "" {
		return ""
	}
	var name strings.Builder
	for i, index := range err.index {
		if index.Name == "" {
			name.WriteByte('[')
			name.WriteString(fmt.Sprint(index.Offset))
			name.WriteByte(']')
			continue
		}
		if i > 0 {
			name.WriteByte('.')
		}
		name.WriteString(index.Name)
	}
	if len(err.index) > 0 {
		name.WriteByte('.')
	}
	return err.field.Name
}

type ErrInvalid struct {
	mirror

	Class string
	Hints string
}

func (err *ErrInvalid) StatusHTTP() int { return 400 }

func (err *ErrInvalid) WithFieldNameSetTo(name string) error {
	var copy = err.mirror
	copy.field.Name = name
	return &ErrInvalid{
		mirror: copy,
		Class:  err.Class,
		Hints:  err.Hints,
	}
}

func (err *ErrInvalid) Error() string {
	if err.FieldName() == "" {
		return fmt.Sprintf("please ensure that all '%s' parameters are valid\n(%s)", err.Class, err.Hints)
	}
	return fmt.Sprintf("please ensure '%s' is valid\n(%s: %s)", err.FieldName(), err.Class, err.Hints)
}

type ErrExceeds struct {
	mirror

	Limit string
}

func (err *ErrExceeds) StatusHTTP() int { return 400 }

func (err *ErrExceeds) WithFieldNameSetTo(name string) error {
	var copy = err.mirror
	copy.field.Name = name
	return &ErrExceeds{
		mirror: copy,
		Limit:  err.Limit,
	}
}

func (err *ErrExceeds) Error() string {
	if err.field.Name == "" {
		return "please ensure that all parameters do not exceed their limits"
	}
	return fmt.Sprintf("please limit '%s' to less than %s", err.field.Name, err.Limit)
}

type ErrMissing struct {
	mirror
}

func (err *ErrMissing) StatusHTTP() int { return 400 }

func (err *ErrMissing) WithFieldNameSetTo(name string) error {
	var copy = err.mirror
	copy.field.Name = name
	return &ErrMissing{
		mirror: copy,
	}
}

func (err *ErrMissing) Error() string {
	if err.field.Name == "" {
		return "please provide all required parameters"
	}
	return fmt.Sprintf("please provide '%s'", err.field.Name)
}

type ReportingNumber[T numeric] struct {
	// Invalid reports an error if the number does not match the given
	// 'format' string. 'hints' are optional and can be used to provide
	// additional information about the specification of the format.
	Invalid func(format string, fn func(T) bool, hints ...string) error
	// Missing reports an error if the field is equal to 0.
	Missing func() error
}

// Number identifies a numeric field to report on.
func Number[T numeric](value *T) ReportingNumber[T] {
	return ReportingNumber[T]{
		Invalid: func(class string, fn func(T) bool, hints ...string) error {
			if !fn(*value) {
				return &ErrInvalid{
					mirror: mirror{value: value},
					Class:  class,
					Hints:  strings.Join(hints, "\n"),
				}
			}
			return nil
		},
		Missing: func() error {
			if *value == 0 {
				return &ErrMissing{
					mirror: mirror{value: value},
				}
			}
			return nil
		},
	}
}

type ReportingString[T ~string | ~[]byte] struct {
	// Invalid reports an error if the number does not match the given
	// 'format' string. 'hints' are optional and can be used to provide
	// additional information about the specification of the format.
	Invalid func(class string, fn func(T) bool, hints ...string) error
	// Exceeds reports an error if the length of the string is greater
	// than the given limit.
	Exceeds func(limit int) error
	// Missing reports an error if the field is equal to the zero value
	// of T.
	Missing func() error
}

// String identifies a string field to validate.
func String[T ~string | ~[]byte](value *T) ReportingString[T] {
	return ReportingString[T]{
		Invalid: func(class string, fn func(T) bool, hints ...string) error {
			if !fn(*value) {
				return &ErrInvalid{
					mirror: mirror{value: value},
					Class:  class,
					Hints:  strings.Join(hints, "\n"),
				}
			}
			return nil
		},
		Exceeds: func(limit int) error {
			if len(*value) > limit {
				return &ErrExceeds{
					mirror: mirror{value: value},
					Limit:  fmt.Sprintf("%d bytes", limit),
				}
			}
			return nil
		},
		Missing: func() error {
			if len(*value) == 0 {
				return &ErrMissing{
					mirror: mirror{value: value},
				}
			}
			return nil
		},
	}
}

type ReportingSliced[T any] struct {
	// Invalid reports an error if the number does not match the given
	// 'format' string. 'hints' are optional and can be used to provide
	// additional information about the specification of the format.
	Invalid func(class string, fn func([]T) bool, hints ...string) error
	// ForEach iterates over each element in the slice and reports an
	// error if the given function returns an error.
	ForEach func(func(*T) error) error
	// Exceeds reports an error if the length of the slice is greater
	// than the given limit.
	Exceeds func(int) error
	// Missing reports an error if the length of the slice is equal to 0.
	Missing func() error
}

// Sliced identifies a slice field to validate.
func Sliced[T any](value *[]T) ReportingSliced[T] {
	return ReportingSliced[T]{
		Invalid: func(class string, fn func([]T) bool, hints ...string) error {
			if !fn(*value) {
				return &ErrInvalid{
					mirror: mirror{value: value},
					Class:  class,
					Hints:  strings.Join(hints, "\n"),
				}
			}
			return nil
		},
		ForEach: func(fn func(*T) error) error {
			for i := range *value {
				if err := fn(&(*value)[i]); err != nil {
					return err
				}
			}
			return nil
		},
		Exceeds: func(limit int) error {
			if len(*value) > limit {
				return &ErrExceeds{
					mirror: mirror{value: value},
					Limit:  fmt.Sprintf("%d items", limit),
				}
			}
			return nil
		},
		Missing: func() error {
			if len(*value) == 0 {
				return &ErrMissing{
					mirror: mirror{value: value},
				}
			}
			return nil
		},
	}
}

type ReportingMapped[K comparable, V any] struct {
	// Invalid reports an error if the number does not match the given
	// 'format' string. 'hints' are optional and can be used to provide
	// additional information about the specification of the format.
	Invalid func(class string, fn func(map[K]V) bool, hints ...string) error
	// Exceeds reports an error if the length of the map is greater
	// than the given limit.
	Exceeds func(int) error
	// ForEach iterates over each element in the map and reports an
	// error if the given function returns an error.
	ForEach func(func(*V) error) error
	// MapKeys iterates over each key in the map and reports an error
	// if the given function returns an error.
	MapKeys func(func(*K) error) error
	// Missing reports an error if the length of the map is equal to 0.
	Missing func() error
}

// Mapped identifies a map field to validate.
func Mapped[K comparable, V any](value *map[K]V) ReportingMapped[K, V] {
	return ReportingMapped[K, V]{
		Invalid: func(class string, fn func(map[K]V) bool, hints ...string) error {
			if !fn(*value) {
				return &ErrInvalid{
					mirror: mirror{value: value},
					Class:  class,
					Hints:  strings.Join(hints, "\n"),
				}
			}
			return nil
		},
		Exceeds: func(limit int) error {
			if len(*value) > limit {
				return &ErrExceeds{
					mirror: mirror{value: value},
					Limit:  fmt.Sprintf("%d items", limit),
				}
			}
			return nil
		},
		ForEach: func(fn func(*V) error) error {
			for _, v := range *value {
				if err := fn(&v); err != nil {
					return err
				}
			}
			return nil
		},
		MapKeys: func(fn func(*K) error) error {
			for k := range *value {
				if err := fn(&k); err != nil {
					return err
				}
			}
			return nil
		},
		Missing: func() error {
			if len(*value) == 0 {
				return &ErrMissing{
					mirror: mirror{value: value},
				}
			}
			return nil
		},
	}
}
