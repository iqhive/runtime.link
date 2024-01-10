//go:build amd64 || arm64

package cpu

import "unsafe"

type register uint64

type floatingPointRegister float64

// Int64 returns the register as an int64.
func (r *Register) Int64() int64 {
	return *(*int64)(unsafe.Pointer(r))
}

// SetInt64 sets the register to i.
func (r *Register) SetInt64(i int64) {
	*r = *(*Register)(unsafe.Pointer(&i))
}

// Uint64 returns the register as an uint64.
func (r *Register) Uint64() uint64 {
	return *(*uint64)(unsafe.Pointer(r))
}

// SetUint64 sets the register to i.
func (r *Register) SetUint64(i uint64) {
	*r = *(*Register)(unsafe.Pointer(&i))
}

// Float64 returns the register as a float64.
func (r *Register) Float64() float64 {
	return *(*float64)(unsafe.Pointer(r))
}

// SetFloat64 sets the register to f.
func (r *Register) SetFloat64(f float64) {
	*r = *(*Register)(unsafe.Pointer(&f))
}

// Int64 returns the register as an int64.
func (r *FloatingPointRegister) Int64() int64 {
	return *(*int64)(unsafe.Pointer(r))
}

// SetInt64 sets the register to i.
func (r *FloatingPointRegister) SetInt64(i int64) {
	*r = *(*FloatingPointRegister)(unsafe.Pointer(&i))
}

// Uint64 returns the register as an uint64.
func (r *FloatingPointRegister) Uint64() uint64 {
	return *(*uint64)(unsafe.Pointer(r))
}

// SetUint64 sets the register to i.
func (r *FloatingPointRegister) SetUint64(i uint64) {
	*r = *(*FloatingPointRegister)(unsafe.Pointer(&i))
}

// Float64 returns the register as a float64.
func (r *FloatingPointRegister) Float64() float64 {
	return *(*float64)(unsafe.Pointer(r))
}

// SetFloat64 sets the register to f.
func (r *FloatingPointRegister) SetFloat64(f float64) {
	*r = *(*FloatingPointRegister)(unsafe.Pointer(&f))
}
