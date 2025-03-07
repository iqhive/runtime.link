package arm64

import (
	"runtime.link/api"
)

type CheckedPointer uintptr
type TaggedPointer uintptr
type ProgramCounter uintptr

type X = Register[[8]byte]

type Register[T uint64 | int64 | [8]byte | uintptr | CheckedPointer | TaggedPointer | ProgramCounter | bool] Uint5

type API struct {
	api.Specification

	ABS  func(dst, src Register[int64]) error `feat:"CSSC"` // ABS computes 'dst=|src|', writing the absolute value of 'src' into 'dst'.
	ADC  func(dst, a, b X) error              // ADC computes 'dst=(a+b)' taking into account the current carry flag.
	ADCS func(dst, a, b X) error              // ADCS computes 'dst=(a+b)' taking into account the current carry flag and sets the NZCV flags.
	ADD  ExtendedImmediateShifted[
		func(dst, a, b X, extension RegisterExtension, amount Uint3) error,
		func(dst, a X, b Imm12) error,
		func(dst, a, b X, shift Shift, amount Uint6) error,
	] // Add computes 'dst=(a+b)', with 'b' either being an immediate values or a bit-shifted/extended register.
	ADDG  func(dst, src Register[TaggedPointer], offset Int6, tag_offset Int4) error                `feat:"MTE"` // ADDG adds an offset to the address of the tagged pointer 'src', adds a tag_offset to its tag and writes the resulting tagged pointer to 'dst'.
	ADDPT func(dst, src Register[CheckedPointer], offset Register[int64], shift_amount Uint3) error `feat:"CPA"` // ADDPT adds an optionally shifted 'offset' to the checked pointer 'src' and writes the resulting checked pointer to 'dst'.
	ADDS  ExtendedImmediateShifted[
		func(dst, a, b X, extension RegisterExtension, amount Uint3) error,
		func(dst, a X, immediate Uint12) error,
		func(dst, a, b X, shift Shift, amount Uint6) error,
	] // ADD computes 'dst=(a+b)' and sets the NZCV flag, with 'b' either being an immediate values or a bit-shifted/extended register.
	ADR  func(dst Register[ProgramCounter], offset Int21) error // ADR adds an offset to the current program counter (PC) and writes the resulting address to 'dst'.
	ADRP func(dst Register[ProgramCounter], offset Int21) error // ADRP adds an offset to the current program counter (PC) and writes the 4KB aligned resulting address to 'dst'.
	AND  ImmediateShifted[
		func(dst, src X, val BitPattern) error,
		func(dst, a, b X, shift Shift, amount Uint6) error,
	] // AND computes 'dst=(a&b)', with 'b' either being an immediate bit pattern or a bit-shifted register.
	ANDS ImmediateShifted[
		func(dst, src X, val BitPattern) error,
		func(dst, a, b X, shift Shift, amount Uint6) error,
	] // ANDS computes 'dst=(a&b)' and sets the NZCV flags, with 'b' either being an immediate bit pattern or a bit-shifted register.
	APAS func(src Register[uintptr]) error `feat:"RME_GPC3"` // APAS associates a physical address space with a memory-mapped location that is protected by a memory-side physical address space filter.
	ASR  ImmediateRegister[
		func(dst, a Register[int64], b Uint6) error,
		func(dst, a Register[int64], b Register[uint64]) error,
	] // ASR computes 'dst=(src>>amount)' writing the result to 'dst'.
	AT          func(ptr Register[uintptr], stage Stage, exception_level Uint2, checks AddressChecks) error // AT performs address translation and writes the translated address into the physical address register.
	AUTDA       func(dst Register[uintptr], key X) error                                                    `feat:"PAuth"`    // AUTDA authenticates the 'dst' address in-place using the given A 'key' modifier, which must be the same used to create the pointer (ie. with PACDA). If the authentication fails, the pointer will fault on use.
	AUTDZA      func(dst Register[uintptr]) error                                                           `feat:"PAuth"`    // AUTDZA A authenticates the 'dst' address in-place. If the authentication fails, the pointer will fault on use.
	AUTDB       func(dst Register[uintptr], key X) error                                                    `feat:"PAuth"`    // AUTDB authenticates the 'dst' address in-place using the given B 'key' modifier, which must be the same used to create the pointer (ie. with PACDB). If the authentication fails, the pointer will fault on use.
	AUTDZB      func(dst Register[uintptr]) error                                                           `feat:"PAuth"`    // AUTDZB authenticates the 'dst' address in-place. If the authentication fails, the pointer will fault on use.
	AUTIA       func(dst Register[ProgramCounter], key X) error                                             `feat:"PAuth"`    // AUTIA authenticates the 'dst' code address in-place using the given A 'key' modifier, which must be the same used to create the pointer (ie. with PACIA). If the authentication fails, the pointer will fault on use.
	AUTIA1716   func() error                                                                                `feat:"PAuth"`    // AUTIA1716 authenticates X17 code address in-place using the given A X16 modifier (and X15 modified if PSTATE.PACM is 1), which must be the same used to create the pointer (ie. with PACIA). If the authentication fails, the pointer will fault on use.
	AUTIASP     func() error                                                                                `feat:"PAuth"`    // AUTIASP authenticates X30 in-place using the stack pointer (and X16 if PSTATE.PACM is 1) as the A modifier, which must be the same used to create the pointer (ie. with PACIA). If the authentication fails, the pointer will fault on use.
	AUTIAZ      func() error                                                                                `feat:"PAuth"`    // AUTIAZ authenticates the 'dst' code address in-place. If the authentication fails, the pointer will fault on use.
	AUTIZA      func(dst Register[ProgramCounter]) error                                                    `feat:"PAuth"`    // AUTIZA authenticates the 'dst' code address in-place. If the authentication fails, the pointer will fault on use.
	AUTIA171615 func() error                                                                                `feat:"PAuth_LR"` // AUTIA171615 authenticates X17 code address in-place using the given A X16 and X15 modifiers, which must be the same used to create the pointer (ie. with PACIA). If the authentication fails, the pointer will fault on use.
	AUTIASPPC   func(pc_offset int16) error                                                                 `feat:"PAuth_LR"` // AUTIASPPC authenticates the X30 code address in-place using the program counter offset A and stack pointer as modifiers, which must be the same used to create the pointer (ie. with PACIA). If the authentication fails, the pointer will fault on use.
	AUTIASPPCR  func(key X) error                                                                           `feat:"PAuth_LR"` // AUTIASPPCR authenticates the X30 code address in-place using the given 'key' and stack pointer as A modifiers, which must be the same used to create the pointer (ie. with PACIA). If the authentication fails, the pointer will fault on use.
	AUTIB       func(dst Register[ProgramCounter], key X) error                                             `feat:"PAuth"`    // AUTIB authenticates the 'dst' code address in-place using the given B 'key' modifier, which must be the same used to create the pointer (ie. with PACIB). If the authentication fails, the pointer will fault on use.
	AUTIB1716   func() error                                                                                `feat:"PAuth"`    // AUTIB1716 authenticates X17 code address in-place using the given B X16 modifier (and X15 modified if PSTATE.PACM is 1), which must be the same used to create the pointer (ie. with PACIB). If the authentication fails, the pointer will fault on use.
	AUTIBSP     func() error                                                                                `feat:"PAuth"`    // AUTIBSP authenticates X30 in-place using the stack pointer (and X16 if PSTATE.PACM is 1) as the B modifier, which must be the same used to create the pointer (ie. with PACIB). If the authentication fails, the pointer will fault on use.
	AUTIZB      func(dst Register[ProgramCounter]) error                                                    `feat:"PAuth"`    // AUTIZB authenticates the 'dst' code address in-place. If the authentication fails, the pointer will fault on use.
	AUTIBZ      func() error                                                                                `feat:"PAuth"`    // AUTIBZ authenticates the X30 code address in-place. If the authentication fails, the pointer will fault on use.
	AUTIB171615 func() error                                                                                `feat:"PAuth_LR"` // AUTIB171615 authenticates X17 code address in-place using the given B X16 and X15 modifiers, which must be the same used to create the pointer (ie. with PACIB). If the authentication fails, the pointer will fault on use.
	AUTIBSPPC   func(pc_offset int16) error                                                                 `feat:"PAuth_LR"` // AUTIBSPPC authenticates the X30 code address in-place using the program counter offset B and stack pointer as modifiers, which must be the same used to create the pointer (ie. with PACIB). If the authentication fails, the pointer will fault on use.
	AUTIBSPPCR  func(key X) error                                                                           `feat:"PAuth_LR"` // AUTIBSPPCR authenticates the X30 code address in-place using the given 'key' and stack pointer as B modifiers, which must be the same used to create the pointer (ie. with PACIB). If the authentication fails, the pointer will fault on use.
	AXFLAG      func() error                                                                                `feat:"FlagM2"`   // AXFLAG converts the NZCF flags from a form representing the result of an Arm floating-point scalar compare instruction to an alternative representation required by some software.

	CMPs func(src1, src2 X, shift Shift, amount Uint6) error // Compare and Set Flags
	CSET func(dst Register[bool], condition Condition) error // Conditional Set
	RET  func(lnk Register[ProgramCounter]) error            // Return
}

// ASRV is an alias for [API.ASR.Register].
func (arm64 API) ASRV(dst, a Register[int64], b Register[uint64]) error {
	return arm64.ASR.Register(dst, a, b)
}

type Int21 int32
type Int12 int16
type Int6 int8
type Int4 int8
type Int3 int8

// Immediate types may be signed or unsigned depending on the
// context in which they are used.
type (
	Imm12 int16
	Imm13 int16
)

type Uint21 uint32
type Uint16 uint16
type Uint12 uint16
type Uint13 uint16
type Uint6 uint8
type Uint5 uint8
type Uint4 uint8
type Uint3 uint8
type Uint2 uint8

// BitPattern represents a 64-bit value formed by repeating a contiguous block of ones,
// wrap-around shifted right aka rotated within elements of equal size (2, 4, 8, 16, 32,
// or 64 bits), as encoded by ARM64 logical immediates using n, immr, and imms.
type BitPattern uint64

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

type ImmediateShifted[I, S any] struct {
	Immediate       I
	ShiftedRegister S
}

func NewImmediateShifted[I, S any](i I, s S) ImmediateShifted[I, S] {
	return ImmediateShifted[I, S]{Immediate: i, ShiftedRegister: s}
}

type ImmediateRegister[I, R any] struct {
	Immediate I
	Register  R
}

func NewImmediateRegister[I, R any](i I, r R) ImmediateRegister[I, R] {
	return ImmediateRegister[I, R]{Immediate: i, Register: r}
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

type Stage Uint2

const (
	Stage1 Stage = 0b01 // Stage 1
	Stage2 Stage = 0b10 // Stage 2
)

type AddressChecks Uint2

const (
	AddressCheckRead           AddressChecks = 0b001
	AddressCheckWrite          AddressChecks = 0b000
	AddressCheckAuthentication AddressChecks = 0b010 // feat:"PAN2"
	AddressCheckAlignment      AddressChecks = 0b100 // feat:"ATS1A"
)
