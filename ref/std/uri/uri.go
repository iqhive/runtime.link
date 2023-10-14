// Package uri provides URI reference types.
package uri

import (
	"net/url"
)

// String is a URI.
type String string

// Validate implements [has.Validation]
func (uri String) Validate() error {
	_, err := url.Parse(string(uri))
	return err
}
