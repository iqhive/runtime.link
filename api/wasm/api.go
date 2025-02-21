package wasm

import (
	"io"
)

type (
	ref_library  uint32
	ref_function uint32

)

type link_interface struct {
	dlopen func(io.Reader, int) ref_library
	dlsym  func(ref_library, io.Reader, int) ref_function


}
