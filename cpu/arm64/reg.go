package arm64

import "unsafe"

type TaggedPointer uintptr
type CheckedPointer uintptr

type RegisterValue64 interface {
	uint64 | int64 | float64 | unsafe.Pointer | uintptr | TaggedPointer | CheckedPointer
}

type RegisterValue32 interface {
	uint32 | int32 | float32 | uint16 | int16 | uint8 | int8
}

type anyRegister interface {
	anyVector | X[uint64] | W[uint32] |
		D[uint64] | D[float64] | D[int64] | D[uintptr] | D[unsafe.Pointer] | D[TaggedPointer] | D[CheckedPointer] |
		S[uint32] | S[float32] | S[int32] |
		H[uint16] | H[int16] |
		B[uint8] | B[int8] |
		W[int32] | X[int64] |
		W[float32] | X[float64] |
		X[uintptr] | X[unsafe.Pointer] |
		X[TaggedPointer] | X[CheckedPointer]
}

type PurposeGeneral struct{}
type PurposeVectors struct{}

type Int21 int32
type Int12 int16
type Int6 int8
type Int4 int8
type Int3 int8

type Uint21 uint32
type Uint12 uint16
type Uint6 uint8
type Uint4 uint8
type Uint3 uint8

// ZR is the zero register.
const ZR = 31

type V[
	T [16]uint8 | [16]int8 | [8]uint16 | [8]int16 | [4]uint32 | [4]int32 |
		[4]float32 | [2]uint64 | [2]float64 | [2]int64 | [2]uintptr | [2]unsafe.Pointer |
		[2]TaggedPointer | [2]CheckedPointer | [2]complex64 | complex128,
] uint8 // Nth 128-bit SIMD and floating-point register (0-31)

type anyVector interface {
	V[[16]uint8] | V[[16]int8] | V[[8]uint16] | V[[8]int16] | V[[4]uint32] | V[[4]int32] |
		V[[4]float32] | V[[2]uint64] | V[[2]float64] | V[[2]int64] | V[[2]uintptr] | V[[2]unsafe.Pointer] |
		V[[2]TaggedPointer] | V[[2]CheckedPointer] | V[[2]complex64] | V[complex128]
}

type X[T RegisterValue64] uint8 // X is a 64-bit general-purpose register (0-30)
type W[T RegisterValue32] uint8 // W is a 32-bit general-purpose register (0-30).

type D[T RegisterValue64] uint8 // D is a 64-bit SIMD and floating-point register (0-31).
type S[T RegisterValue32] uint8 // S is a 32-bit SIMD and floating-point register (0-31).
type H[T uint16 | int16] uint8  // H is a 16-bit SIMD and floating-point register (0-31).
type B[T uint8 | int8] uint8    // B is an 8-bit SIMD and floating-point register (0-31).
