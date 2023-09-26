package lib

import (
	"errors"

	"runtime.link/qnq"
)

// Location can be added to a library structure to specify
// the standard location or name of that library on a
// specific GOOS.
//
// For example:
//
//	type Library struct {
//		linux   lib.Location `lib:"libc.so.6 libm.so.6"`
//		darwin  lib.Location `lib:"libSystem.dylib"`
//		windows lib.Location `lib:"msvcrt.dll"`
//	}
type Location struct{}

// Documentation can be embedded into a runtime.link structure to indicate that
// it supports the shared library link layer.
type Documentation struct {
	qnq.Host
}

// Make generates bindings for the given library in the given
// directory.
func Make(dir string, functions any) error {
	return errors.New("library generation not yet implemented")
}
