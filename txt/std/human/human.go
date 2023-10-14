// Package human provides standard way to reference people by name.
package human

import (
	"fmt"
	"unicode"
)

// Name refers to a person by their name.
type Name string

// Validate implements [has.Validation]
func (name Name) Validate() error {
	for _, char := range name {
		if !unicode.IsLetter(char) && char != ' ' {
			return fmt.Errorf("invalid character in name: %q", char)
		}
	}
	return nil
}

// Readable annotates a string as being readable by a person.
type Readable string

// Validate implements [has.Validation]
func (readable Readable) Validate() error {
	for _, char := range readable {
		if unicode.IsPrint(char) {
			return fmt.Errorf("unreadable character: %q", char)
		}
	}
	return nil
}
