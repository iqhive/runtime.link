package link

import (
	"errors"

	"runtime.link/api"
)

// Location can be added to a library structure to specify
// the standard location or name of that library on a
// specific GOOS.
//
// For example:
//
//	type Library struct {
//		linux   link.Library `link:"libc.so.6 libm.so.6"`
//		darwin  link.Library `link:"libSystem.dylib"`
//		windows link.Library `link:"msvcrt.dll"`
//	}
type Library struct{}

// Documentation can be embedded into a runtime.link structure to indicate that
// it supports the shared library link layer.
type Documentation struct {
	api.Host
}

// Make generates bindings for the given library in the given
// directory.
func Make(dir string, functions any) error {
	return errors.New("library generation not yet implemented")
}
