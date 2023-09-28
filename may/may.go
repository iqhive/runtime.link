// Package may provides a way to represent optional 'maybe' values for struct fields and function parameters.
package may

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"runtime.link/txt"
)

type (
	Prefix                  bool
	Contain                 string
	Backtick[T txt.Matcher] struct {
		WithBacktick T
	}
)

func (p Prefix) String() string { return strconv.FormatBool(bool(p)) }

func (p Prefix) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	if strings.HasPrefix(raw, string(tag)) {
		*(ptr.(*Prefix)) = true
		return len(tag), nil
	}
	*(ptr.(*Prefix)) = false
	return 0, nil
}

func (c Contain) String() string { return string(c) }
func (Contain) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	contains := ptr.(*Contain)
	for _, char := range raw {
		if !strings.ContainsRune(string(tag), char) {
			return 0, fmt.Errorf("invalid '%v' character", string(char))
		}
		*contains += Contain(char)
	}
	return 0, nil
}

func (b Backtick[T]) String() string { return b.WithBacktick.String() }
func (Backtick[T]) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	val := *(ptr.(*Backtick[T]))
	return val.WithBacktick.MatchString(&val.WithBacktick, raw, tag+"`")
}

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
