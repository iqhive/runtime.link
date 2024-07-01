// Package ffi provides representations for working with immutable foreign data structures.
package ffi

import (
	"unsafe"

	"runtime.link/mmm"
	"runtime.link/xyz"
)

type sized interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64 | ~uintptr |
		~[0]int8 | ~[0]int16 | ~[0]int32 | ~[0]int64 | ~[0]uint8 | ~[0]uint16 | ~[0]uint32 | ~[0]uint64 | ~[0]float32 | ~[0]float64 | ~[0]uintptr |
		~[1]int8 | ~[1]int16 | ~[1]int32 | ~[1]int64 | ~[1]uint8 | ~[1]uint16 | ~[1]uint32 | ~[1]uint64 | ~[1]float32 | ~[1]float64 | ~[1]uintptr |
		~[2]int8 | ~[2]int16 | ~[2]int32 | ~[2]int64 | ~[2]uint8 | ~[2]uint16 | ~[2]uint32 | ~[2]uint64 | ~[2]float32 | ~[2]float64 | ~[2]uintptr |
		~[3]int8 | ~[3]int16 | ~[3]int32 | ~[3]int64 | ~[3]uint8 | ~[3]uint16 | ~[3]uint32 | ~[3]uint64 | ~[3]float32 | ~[3]float64 | ~[3]uintptr |
		~[4]int8 | ~[4]int16 | ~[4]int32 | ~[4]int64 | ~[4]uint8 | ~[4]uint16 | ~[4]uint32 | ~[4]uint64 | ~[4]float32 | ~[4]float64 | ~[4]uintptr
}

type array interface {
	pointer

	Len() int
	Cap() int
}

type pointer interface {
	UnsafePointer() unsafe.Pointer
}

// Slice is an immutable slice of fixed-size elements.
type Slice[T sized] struct {
	Interface array
}

func (slice Slice[T]) Index(i int) T {
	ptr := slice.Interface.UnsafePointer()
	len := slice.Interface.Len()
	if ptr == nil {
		panic("nil pointer")
	}
	if i < 0 || i >= len {
		panic("index out of range")
	}
	var zero T
	return *(*T)(unsafe.Add(ptr, uintptr(i)*unsafe.Sizeof(zero)))
}

func (slice Slice[T]) Len() int {
	return slice.Interface.Len()
}

// String is a zero-terminated C-style string.
type String struct {
	Interface pointer
}

// Bytes is an immutable slice of bytes.
type Bytes Slice[byte]

func (bytes Bytes) Len() int {
	return bytes.Interface.Len()
}

func (bytes Bytes) Index(i int) byte {
	return Slice[byte](bytes).Index(i)
}

// Managed is an immutable value with a Go representation of T.
type Managed[T any] struct {
	state *managedState[T]
}

type managedState[T any] struct {
	cache any

	lifetime mmm.Lifetime
	value    xyz.Maybe[T]
}

func (managed *Managed[T]) Lifetime() mmm.Lifetime {
	if managed.state == nil {
		managed.state = new(managedState[T])
	}
	if managed.state.lifetime == (mmm.Lifetime{}) {
		managed.state.lifetime = mmm.NewLifetime()
	}
	return managed.state.lifetime
}

func (managed *Managed[T]) Cache(value any) {
	if managed.state == nil {
		managed.state = new(managedState[T])
	}
	managed.state.cache = value
}

func (managed *Managed[T]) Value() T {
	if managed.state == nil {
		var zero T
		return zero
	}
	value, ok := managed.state.value.Get()
	if !ok {
		if managed.state.cache == nil {
			var zero T
			return zero
		}
	}
	return value
}

func (managed *Managed[T]) Interface() any {
	if managed.state == nil {
		return nil
	}
	return managed.state.cache
}

type Pointers[API any, T mmm.PointerWithFree[API, T, Size], Size mmm.PointerSize] struct {
	API *API
	Slice[Size]
}
