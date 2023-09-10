// Package asm provides access to assembly routines for calling C functions from Go.
package asm

import "unsafe"

/*
This package observes the Go ABI0 calling convention for amd64 and arm64, so that
C functions can be called from Go without CGO. Until this is stable, each new Go
version and compiler will need to be checked for compatibility.

https://go.googlesource.com/go/+/refs/heads/master/src/cmd/compile/abi-internal.md
*/
type Registers struct {
	registers

	i, f uint8 // counts
}

func (r *Registers) PushF64(v float64) {
	if r.f < maxFloating {
		*(*float64)(unsafe.Pointer(uintptr(unsafe.Pointer(r)) + offsetFloating + uintptr(r.f)*8)) = v
		r.f++
	}
}

func (r *Registers) PushU64(v uint64) {
	if r.i < maxIntegers {
		*(*uint64)(unsafe.Pointer(uintptr(unsafe.Pointer(r)) + offsetIntegers + uintptr(r.i)*8)) = v
		r.i++
	}
}

// PullI64 starts with an empty and moves the counter up
// pulling the next integer from the registers.
func (r *Registers) PullI64() uint64 {
	r.i++
	return *(*uint64)(unsafe.Pointer(uintptr(unsafe.Pointer(r)) + offsetIntegers + uintptr(r.i-1)*8))
}

// PullF64 starts with an empty and moves the counter up
// pulling the next float from the registers.
func (r *Registers) PullF64() float64 {
	r.f++
	return *(*float64)(unsafe.Pointer(uintptr(unsafe.Pointer(r)) + offsetFloating + uintptr(r.f-1)*8))
}

// Call calls the C function using the given registers.
// Push must be called before this routine.
func Call(fn unsafe.Pointer, r Registers) Registers {
	push(fn)
	closure := &fn
	result := (*(*func(Registers) Registers)(unsafe.Pointer(&closure)))(r) // no hands!
	result.f = 0
	result.i = 0
	return result
}

func push(unsafe.Pointer)
