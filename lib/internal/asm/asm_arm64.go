package asm

import "unsafe"

const (
	offsetIntegers = unsafe.Offsetof(Registers{}.R0)
	offsetFloating = unsafe.Offsetof(Registers{}.F0)
	maxIntegers    = 8
	maxFloating    = 8
)

// registers only go up to R7 and F7 because these are
// the registers used for calling functions on arm64.
type registers struct {
	R0 uint64
	R1 uint64
	R2 uint64
	R3 uint64
	R4 uint64
	R5 uint64
	R6 uint64
	R7 uint64
	R8 uint64 // caller provides a pointer here for return values larger than a single register.

	F0 float64
	F1 float64
	F2 float64
	F3 float64
	F4 float64
	F5 float64
	F6 float64
	F7 float64
}
