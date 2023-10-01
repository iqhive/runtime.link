package link

import (
	"errors"

	"runtime.link/api"
)

// To can be added to a library structure to specify
// the standard location or name of that library on a
// specific GOOS.
//
// For example:
//
//	type Library struct {
//		linux   link.To `lib:"libc.so.6 libm.so.6"`
//		darwin  link.To `lib:"libSystem.dylib"`
//		windows link.To `lib:"msvcrt.dll"`
//	}
type To struct {
	api.Host
}

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
