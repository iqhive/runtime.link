package abi

import "unsafe"

type memory[T unsafe.Pointer | uintptr] struct {
	addr T
	free func()
}

// Pointer to a known fixed-size value.
type Pointer[T any] struct {
	_   [0]*T
	mem *unsafe.Pointer
}

// Uintptr is an opaque pointer-like value
// that refers to a foreign object. May or
// may not be an actual pointer.
type Uintptr[T any] struct {
	_   [0]*T
	mem *uintptr
}

// String is a null-terminated string value.
type String struct {
	_   [0]*byte
	ptr *unsafe.Pointer
}

type File Uintptr[File]

type JumpBuffer Uintptr[JumpBuffer]
type (
	FilePosition Uintptr[FilePosition]
)
