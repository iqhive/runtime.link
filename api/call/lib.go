package call

import (
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
