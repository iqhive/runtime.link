package arm64

import (
	"runtime.link/api"
)

type X Uint5

type API struct {
	api.Specification

	ABS  func(dst, src X) error  `feat:"cssc"` // Absolute Value
	ADC  func(dst, a, b X) error // Add with Carry
	ADCS func(dst, a, b X) error // Add with Carry and Set Flags
	ADD  ExtendedImmediateShifted[
		func(dst, a, b X, extension RegisterExtension, amount Uint3) error,
		func(dst, a X, immediate Uint12) error,
		func(dst, a, b X, shift Shift, amount Uint6) error,
	] // Add
	ADDG  func(dst, src X, offset Int6, tag_offset Int4) error `feat:"mte"` // Add Tagged Pointer
	ADDPT func(dst, src, offset X, shift_amount Uint3) error   `feat:"cpa"` // Add Checked Pointer
	ADDS  ExtendedImmediateShifted[
		func(dst, a, b X, extension RegisterExtension, amount Uint3) error,
		func(dst, a X, immediate Uint12) error,
		func(dst, a, b X, shift Shift, amount Uint6) error,
	] // Add and Set Flags
	CMPs func(src1, src2 X, shift Shift, amount Uint6) error // Compare and Set Flags
	CSET func(dst X, condition Condition) error              // Conditional Set
	RET  func(lnk X) error                                   // Return
}

type Int21 int32
type Int12 int16
type Int6 int8
type Int4 int8
type Int3 int8

type Uint21 uint32
type Uint16 uint16
type Uint12 uint16
type Uint6 uint8
type Uint5 uint8
type Uint4 uint8
type Uint3 uint8
type Uint2 uint8

// ZR is the zero register.
const ZR = 31

type ExtendedImmediateShifted[X, I, S any] struct {
	ExtendedRegister X
	Immediate        I
	ShiftedRegister  S
}

func NewExtendedImmediateShifted[X, I, S any](x X, i I, s S) ExtendedImmediateShifted[X, I, S] {
	return ExtendedImmediateShifted[X, I, S]{ExtendedRegister: x, Immediate: i, ShiftedRegister: s}
}

// Shift for register shift
type Shift uint8

const (
	ShiftLogicalLeft     Shift = 0 // Logical Left Shift
	ShiftLogicalRight    Shift = 1 // Logical Right Shift
	ShiftArithmeticRight Shift = 2 // Arithmetic Right Shift
	ShiftRotateRight     Shift = 3 // Rotate Right
)

// RegisterExtension for extended register form
type RegisterExtension uint8

const (
	ExtendUnsignedByte     RegisterExtension = 0 // Unsigned Extend Byte
	ExtendUnsignedHalfword RegisterExtension = 1 // Unsigned Extend Halfword
	ExtendUnsignedWord     RegisterExtension = 2 // Unsigned Extend Word (also LSL for 32-bit)
	ExtendUnsignedX        RegisterExtension = 3 // Unsigned Extend X (no-op for 64-bit)
	ExtendSignedByte       RegisterExtension = 4 // Signed Extend Byte
	ExtendSignedHalfword   RegisterExtension = 5 // Signed Extend Halfword
	ExtendSignedWord       RegisterExtension = 6 // Signed Extend Word
	ExtendSignedX          RegisterExtension = 7 // Signed Extend X (no-op for 64-bit)
)

type Condition uint8

const (
	NotEqual Condition = 0b0000 // Not Equal
	Equal    Condition = 0b0001 // Equal
	CarrySet Condition = 0b0011 // Carry Set
)
