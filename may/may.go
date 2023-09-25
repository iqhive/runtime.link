// Package may provides a way to represent optional 'maybe' values for struct fields and function parameters.
package may

import "encoding/json"

// Omit represents a value that is optional and can be omitted from the
// struct or function call it resides within. Not suitable for use as an
// underlying type.
type Omit[T any] map[ok]T

type ok struct{}

// Include returns an un-omitted (included) [Omit] value.
// Calls to [Get] will return this value and true.
func Include[T any](val T) Omit[T] {
	var omit = make(map[ok]T)
	omit[ok{}] = val
	return omit
}

// Get returns the value and true if the value was included
// otherwise it returns the zero value and false.
func (o Omit[T]) Get() (T, bool) {
	val, ok := o[ok{}]
	return val, ok
}

// MarshalJSON implements the [json.Marshaler] interface.
func (o Omit[T]) MarshalJSON() ([]byte, error) {
	if val, ok := o.Get(); ok {
		return json.Marshal(val)
	}
	return []byte("null"), nil
}

// UnmarshalJSON implements the [json.Unmarshaler] interface.
func (o *Omit[T]) UnmarshalJSON(b []byte) error {
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
		*o = Include(val)
	} else {
		(*o)[ok{}] = val
	}
	return nil
}
