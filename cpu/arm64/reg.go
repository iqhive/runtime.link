package arm64

import "unsafe"

type TaggedPointer uintptr
type CheckedPointer uintptr

type Register[P PurposeGeneral | PurposeVectors, T RegisterValue] uint8
type Reg[T RegisterValue] = Register[PurposeGeneral, T]

type RegisterValue interface {
	uint8 | uint16 | uint32 | uint64 | int8 | int16 | int32 | int64 | float32 | float64 | unsafe.Pointer | uintptr | TaggedPointer | CheckedPointer
}

type anyRegister interface {
	X | W | D | S | H | B | anyVector |
		Reg[uint8] | Reg[uint16] |
		Reg[int8] | Reg[int16] |
		Reg[int32] | Reg[int64] |
		Reg[float32] | Reg[float64] |
		Reg[uintptr] | Reg[unsafe.Pointer] |
		Reg[TaggedPointer] | Reg[CheckedPointer]
}

type PurposeGeneral struct{}
type PurposeVectors struct{}

type Imm12 uint16
type Imm6 uint8
type Imm4 uint8
type Imm3 uint8

// XZR is the zero register.
const XZR X = 31

// WZR is the zero register.
const WZR W = 31

type Vector[
	T [16]uint8 | [8]uint16 | [4]uint32 | [2]uint64,
] uint8 // Nth 128-bit SIMD and floating-point register (0-31)

type anyVector interface {
	Vector[[2]uint64] | Vector[[4]uint32] | Vector[[8]uint16] | Vector[[16]uint8]
}

type X = Reg[uint64] // X is a 64-bit general-purpose register (0-30)
type W = Reg[uint32] // W is a 32-bit general-purpose register (0-30).

type D = Register[PurposeVectors, uint64] // D is a 64-bit SIMD and floating-point register (0-31).
type S = Register[PurposeVectors, uint32] // S is a 32-bit SIMD and floating-point register (0-31).
type H = Register[PurposeVectors, uint16] // H is a 16-bit SIMD and floating-point register (0-31).
type B = Register[PurposeVectors, uint8]  // B is an 8-bit SIMD and floating-point register (0-31).
