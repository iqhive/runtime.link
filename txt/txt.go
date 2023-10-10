// Package txt provides text validation types, null terminated strings and a human readable textual struct tag.
package txt

import (
	"encoding"
	"fmt"
)

// Is denotes a string that should always contain values of
// the specified type. The type's pointer reciever should
// implement [encoding.TextUnmarshaler] or 'Parse(string) (*T, error)'
type Is[T any] string

// Get returns the underlying string value if it matches T. Otherwise
// it returns an empty string and a non-nil error.
func (val Is[T]) Get() (string, error) {
	var t T
	switch kind := any(&t).(type) {
	case encoding.TextUnmarshaler:
		if err := kind.UnmarshalText([]byte(val)); err != nil {
			return "", err
		}
	case interface {
		Parse(string) (*T, error)
	}:
		_, err := kind.Parse(string(val))
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("invalid txt.Is type: %v", kind)
	}
	return string(val), nil
}
