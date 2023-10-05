package xyz

import (
	"sync"
	"unsafe"
)

// Ptr to a value of type T, that is not managed by the
// Go runtime. May or may not be mutable.
type Ptr[T any] struct {
	_ [0]*T
	*pointer
}

type pointer struct {
	data unsafe.Pointer
	free func()
	lock sync.RWMutex // we require a mutex for safety.
	edit bool         // mutable? or requires copy ?
}

// Import data from the specified null-terminated memory
// address, if length is provided, no more than length bytes will be read
// from the address. The free function will be called when the
// [Ptr.Free] method is called. If [edit] is true, the string will be
// internally marked as mutable and will be copied when [Ptr.String] is
// called. If the size is not known, pass -1.
func (ptr *Ptr[T]) Import(data unsafe.Pointer, edit bool, free func()) {
	ptr.pointer = &pointer{
		data: data,
		edit: edit,
	}
	ptr.free = func() {
		ptr.lock.Lock()
		defer ptr.lock.Unlock()
		ptr.free = nil
		ptr.data = nil
		free()
	}
}

// Mut returns true if the pointer is mutable.
func (ptr Ptr[T]) Mut() bool {
	ptr.lock.RLock()
	defer ptr.lock.RUnlock()
	return ptr.edit
}

// UnsafePointer takes ownership of the pointer and returns
// a pointer to the underlying memory.
func (ptr Ptr[T]) UnsafePointer() unsafe.Pointer {
	ptr.lock.Lock()
	defer ptr.lock.Unlock()
	addr := ptr.data
	ptr.data = nil
	return unsafe.Pointer(addr)
}

// Free any foreign memory associated with the pointer setting
// it to nil. Future usage of the [Ptr] may result in a panic.
func (ptr Ptr[T]) Free() {
	ptr.lock.RLock()
	defer ptr.lock.RUnlock()
	if ptr.data == nil {
		return
	}
	ptr.free()
}
