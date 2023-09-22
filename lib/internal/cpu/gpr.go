package cpu

import "unsafe"

// Register represents a handle to a general purpose data register.
type Register register

// Bool returns true if the register is not equal to 0.
func (r *Register) Bool() bool {
	return *r != 0
}

// SetBool sets the register to 1 if b is true, otherwise 0.
func (r *Register) SetBool(b bool) {
	if b {
		*r = 1
	} else {
		*r = 0
	}
}

// Int8 returns the register as an int8.
func (r *Register) Int8() int8 {
	return *(*int8)(unsafe.Pointer(r))
}

// SetInt8 sets the register to i.
func (r *Register) SetInt8(i int8) {
	*r = *(*Register)(unsafe.Pointer(&i))
}

// Int32 returns the register as an int32.
func (r *Register) Int32() int32 {
	return *(*int32)(unsafe.Pointer(r))
}

// SetInt32 sets the register to i.
func (r *Register) SetInt32(i int32) {
	*r = *(*Register)(unsafe.Pointer(&i))
}

// Int returns the register as an int.
func (r *Register) Int() int {
	return *(*int)(unsafe.Pointer(r))
}

// SetInt sets the register to i.
func (r *Register) SetInt(i int) {
	*r = *(*Register)(unsafe.Pointer(&i))
}

// Uint8 returns the register as an uint8.
func (r *Register) Uint8() uint8 {
	return *(*uint8)(unsafe.Pointer(r))
}

// SetUint8 sets the register to i.
func (r *Register) SetUint8(i uint8) {
	*r = *(*Register)(unsafe.Pointer(&i))
}

// Int16 returns the register as an int16.
func (r *Register) Int16() int16 {
	return *(*int16)(unsafe.Pointer(r))
}

// SetInt16 sets the register to i.
func (r *Register) SetInt16(i int16) {
	*r = *(*Register)(unsafe.Pointer(&i))
}

// Uint16 returns the register as an uint16.
func (r *Register) Uint16() uint16 {
	return *(*uint16)(unsafe.Pointer(r))
}

// SetUint16 sets the register to i.
func (r *Register) SetUint16(i uint16) {
	*r = *(*Register)(unsafe.Pointer(&i))
}

// Uint32 returns the register as an uint32.
func (r *Register) Uint32() uint32 {
	return *(*uint32)(unsafe.Pointer(r))
}

// SetUint32 sets the register to i.
func (r *Register) SetUint32(i uint32) {
	*r = *(*Register)(unsafe.Pointer(&i))
}

// Uint returns the register as an uint.
func (r *Register) Uint() uint {
	return *(*uint)(unsafe.Pointer(r))
}

// SetUint sets the register to i.
func (r *Register) SetUint(i uint) {
	*r = *(*Register)(unsafe.Pointer(&i))
}

// Uintptr returns the register as an uintptr.
func (r *Register) Uintptr() uintptr {
	return uintptr(*r)
}

// SetUintptr sets the register to i.
func (r *Register) SetUintptr(i uintptr) {
	*r = *(*Register)(unsafe.Pointer(&i))
}

// Float32 returns the register as an float32.
func (r *Register) Float32() float32 {
	return *(*float32)(unsafe.Pointer(r))
}

// SetFloat32 sets the register to i.
func (r *Register) SetFloat32(i float32) {
	*r = *(*Register)(unsafe.Pointer(&i))
}

// UnsafePointer returns the register as an unsafe.Pointer
// only valid when the pointer in this register is runtime
// pinned, static, or when it hasn't been allocated by Go.
func (r *Register) UnsafePointer() unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(r))
}

// SetUnsafePointer sets the register to i.
func (r *Register) SetUnsafePointer(i unsafe.Pointer) {
	*r = *(*Register)(unsafe.Pointer(&i))
}
