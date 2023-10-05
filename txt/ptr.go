package txt

import (
	"bytes"
	"strings"
	"sync"
	"unsafe"
)

// Ptr to a null-terminated string. May point to memory
// not managed by the Go runtime. May or may not be mutable.
type Ptr struct {
	*pointer
}

type pointer struct {
	text *byte
	free func()
	lock sync.RWMutex // foreign? then we require a mutex for safety.
	edit bool         // mutable? or requires copy ?
	size int          // size cache.
}

// New returns a new pointer to a null-terminated string
// containing the first null-terminated string in s, if
// s is not already null-terminated, New will append the
// termination.
func New(s string) Ptr {
	if len(s) == 0 {
		return Ptr{}
	}
	if s[len(s)-1] != 0 {
		s += "\x00"
	}
	ptr := pointer{
		text: unsafe.StringData(s),
		size: len(s),
	}
	return Ptr{pointer: &ptr}
}

// Import null-terminated text from the specified null-terminated memory
// address, if length is provided, no more than length bytes will be read
// from the address. The free function will be called when the
// [Ptr.Free] method is called. If [edit] is true, the string will be
// internally marked as mutable and will be copied when [Ptr.String] is
// called. If the size is not known, pass -1.
func Import(text unsafe.Pointer, size int, edit bool, free func()) Ptr {
	ptr := pointer{
		text: (*byte)(text),
		edit: edit,
		size: size,
	}
	ptr.free = func() {
		ptr.lock.Lock()
		defer ptr.lock.Unlock()
		ptr.free = nil
		ptr.text = nil
		free()
	}
	return Ptr{pointer: &ptr}
}

// Len calculates and returns the length of the string.
func (ptr Ptr) Len() (length int) {
	if ptr.free != nil {
		ptr.lock.RLock()
		defer ptr.lock.RUnlock()
		if ptr.pointer.size >= 0 {
			return ptr.pointer.size
		}
	}
	var (
		raw = unsafe.Pointer(ptr.pointer.text)
	)
	for {
		if *(*byte)(raw) == 0 {
			length = (int(uintptr(raw) - uintptr(unsafe.Pointer(ptr.pointer.text)))) - 1
			break
		}
		raw = unsafe.Add(raw, 1)
	}
	if ptr.free != nil && !ptr.edit {
		ptr.pointer.size = length
	}
	return length
}

// Mut returns true if the pointer is mutable.
func (ptr Ptr) Mut() bool {
	if ptr.free != nil {
		ptr.lock.RLock()
		defer ptr.lock.RUnlock()
	}
	return ptr.edit
}

// UnsafePointer takes ownership of the pointer and returns
// a pointer to the underlying memory.
func (ptr Ptr) UnsafePointer() unsafe.Pointer {
	if ptr.free != nil {
		ptr.lock.Lock()
		defer ptr.lock.Unlock()
		addr := ptr.text
		ptr.text = nil
		return unsafe.Pointer(addr)
	}
	return unsafe.Pointer(ptr.pointer.text)
}

// SetUnsafePointer modifies the pointer to the specified address,
// only valid when the pointer was previously created with
// [Import].
func (ptr Ptr) SetUnsafePointer(addr unsafe.Pointer) {
	if ptr.free == nil {
		panic("txt.Ptr.SetUnsafePointer called on non-imported pointer")
	}
	ptr.lock.Lock()
	defer ptr.lock.Unlock()
	ptr.pointer.text = (*byte)(addr)
}

// String returns the string value of the pointer. If the pointer
// is not marked as mutable, a copy will be returned.
func (ptr Ptr) String() string {
	if ptr.free != nil {
		ptr.lock.RLock()
		defer ptr.lock.RUnlock()
	}
	if ptr.free != nil || ptr.edit {
		s := unsafe.String(ptr.text, ptr.Len())
		buf := strings.Builder{}
		buf.WriteString(s)
		return buf.String()
	}
	return unsafe.String(ptr.text, ptr.Len())
}

// Bytes returns the byte slice value of the pointer. If the pointer
// is not marked as mutable, a copy will be returned.
func (ptr Ptr) Bytes() []byte {
	if ptr.free != nil {
		ptr.lock.RLock()
		defer ptr.lock.RUnlock()
	}
	if ptr.free != nil || ptr.edit {
		s := unsafe.String(ptr.text, ptr.Len())
		buf := bytes.Buffer{}
		buf.WriteString(s)
		return []byte(buf.String())
	}
	return unsafe.Slice(ptr.text, ptr.Len())
}

// Free any foreign memory associated with the pointer setting
// it to nil. Future usage of the [Ptr] may result in a panic.
func (ptr Ptr) Free() {
	if ptr.free != nil {
		ptr.lock.RLock()
		defer ptr.lock.RUnlock()
		if ptr.text == nil {
			return
		}
		ptr.free()
	}
}
