// Package human provides standard way to reference people by name.
package human

import (
	"fmt"
	"unicode"

	"runtime.link/txt"
)

// Name refers to a person by their name.
type Name = txt.Is[name]

type name struct{}

func (name) Parse(text string) (name, error) {
	for _, char := range text {
		if !unicode.IsLetter(char) && char != ' ' {
			return name{}, fmt.Errorf("invalid character in name: %q", char)
		}
	}
	return name{}, nil
}

// Readable annotates a string as being readable by a person.
type Readable = txt.Is[readable]

type readable struct{}

func (readable) Parse(text string) (readable, error) {
	for _, char := range text {
		if unicode.IsPrint(char) {
			return readable{}, fmt.Errorf("unreadable character: %q", char)
		}
	}
	return readable{}, nil
}
