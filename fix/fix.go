// Package fix provides traceable internal errors that require manual intervention to fix when they are raised.
package fix

import (
	"errors"

	"runtime.link/api/xray"
)

type literal string

// This errors needs to be fixed!
func This(err error) error {
	if err == nil {
		return nil
	}
	return xray.Error(err, 1)
}

// Me describes the internal problem that needs to be fixed.
func Me(problem literal) error {
	return xray.Error(errors.New(string(problem)), 1)
}
