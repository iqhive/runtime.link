package cpu

import "unsafe"

// FloatingPointRegister represents a handle to a floating-point optimised data register.
type FloatingPointRegister floatingPointRegister

// Bool returns true if the register is not equal to 0.
func (r *FloatingPointRegister) Bool() bool {
	return *r != 0
}

// SetBool sets the register to 1 if b is true, otherwise 0.
func (r *FloatingPointRegister) SetBool(b bool) {
	if b {
		*r = 1
	} else {
		*r = 0
	}
}

// Int8 returns the register as an int8.
func (r *FloatingPointRegister) Int8() int8 {
	return *(*int8)(unsafe.Pointer(r))
}

// SetInt8 sets the register to i.
func (r *FloatingPointRegister) SetInt8(i int8) {
	*r = *(*FloatingPointRegister)(unsafe.Pointer(&i))
}

// Int32 returns the register as an int32.
func (r *FloatingPointRegister) Int32() int32 {
	return *(*int32)(unsafe.Pointer(r))
}

// SetInt32 sets the register to i.
func (r *FloatingPointRegister) SetInt32(i int32) {
	*r = *(*FloatingPointRegister)(unsafe.Pointer(&i))
}

// Int returns the register as an int.
func (r *FloatingPointRegister) Int() int {
	return *(*int)(unsafe.Pointer(r))
}

// SetInt sets the register to i.
func (r *FloatingPointRegister) SetInt(i int) {
	*r = *(*FloatingPointRegister)(unsafe.Pointer(&i))
}

// Uint8 returns the register as an uint8.
func (r *FloatingPointRegister) Uint8() uint8 {
	return *(*uint8)(unsafe.Pointer(r))
}

// SetUint8 sets the register to i.
func (r *FloatingPointRegister) SetUint8(i uint8) {
	*r = *(*FloatingPointRegister)(unsafe.Pointer(&i))
}

// Uint16 returns the register as an uint16.
func (r *FloatingPointRegister) Uint16() uint16 {
	return *(*uint16)(unsafe.Pointer(r))
}

// SetUint16 sets the register to i.
func (r *FloatingPointRegister) SetUint16(i uint16) {
	*r = *(*FloatingPointRegister)(unsafe.Pointer(&i))
}

// Uint32 returns the register as an uint32.
func (r *FloatingPointRegister) Uint32() uint32 {
	return *(*uint32)(unsafe.Pointer(r))
}

// SetUint32 sets the register to i.
func (r *FloatingPointRegister) SetUint32(i uint32) {
	*r = *(*FloatingPointRegister)(unsafe.Pointer(&i))
}

// Uint returns the register as an uint.
func (r *FloatingPointRegister) Uint() uint {
	return *(*uint)(unsafe.Pointer(r))
}

// SetUint sets the register to i.
func (r *FloatingPointRegister) SetUint(i uint) {
	*r = *(*FloatingPointRegister)(unsafe.Pointer(&i))
}

// Uintptr returns the register as an uintptr
func (r *FloatingPointRegister) Uintptr() uintptr {
	return *(*uintptr)(unsafe.Pointer(r))
}

// SetUintptr sets the register to i.
func (r *FloatingPointRegister) SetUintptr(i uintptr) {
	*r = *(*FloatingPointRegister)(unsafe.Pointer(&i))
}

// UnsafePointer returns the register as an unsafe.Pointer
// only valid when the pointer in this register is runtime
// pinned, static, or when it hasn't been allocated by Go.
func (r *FloatingPointRegister) UnsafePointer() unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(r))
}

// SetUnsafePointer sets the register to i.
func (r *FloatingPointRegister) SetUnsafePointer(i unsafe.Pointer) {
	*r = *(*FloatingPointRegister)(unsafe.Pointer(&i))
}

// Float32 returns the register as a float32.
func (r *FloatingPointRegister) Float32() float32 {
	return *(*float32)(unsafe.Pointer(r))
}

// SetFloat32 sets the register to f.
func (r *FloatingPointRegister) SetFloat32(f float32) {
	*r = *(*FloatingPointRegister)(unsafe.Pointer(&f))
}
