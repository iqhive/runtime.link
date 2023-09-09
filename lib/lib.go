package lib

import (
	"errors"

	"runtime.link/ffi"
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

// Import the given library, using the additionally provided
// locations to search for the library.
func Import[Library any](locations ...Location) Library {
	var lib Library
	var structure = ffi.StructureOf(&lib)
	structure.MakeError(errors.New("library import not yet implemented"))
	return lib
}

// Make generates bindings for the given library in the given
// directory.
func Make(dir string, functions any) error {
	return errors.New("library generation not yet implemented")
}
