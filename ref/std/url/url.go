// Package url provides URL reference types.
package url

import (
	"net/url"

	"runtime.link/api/xray"
)

// String is a URL.
type String string

// Validate implements [has.Validation]
func (s String) Validate() error {
	_, err := url.Parse(string(s))
	return xray.Error(err)
}
