package arm64

// shift for shifted register form
type shift uint8

const (
	shiftLogicalLeft     shift = 0 // Logical Shift Left
	shiftLogicalRight    shift = 1 // Logical Shift Right
	shiftArithmeticRight shift = 2 // Arithmetic Shift Right
)

// registerExtension for extended register form
type registerExtension uint8

const (
	extendUnsignedByte     registerExtension = 0 // Unsigned Extend Byte
	extendUnsignedHalfword registerExtension = 1 // Unsigned Extend Halfword
	extendUnsignedWord     registerExtension = 2 // Unsigned Extend Word (also LSL for 32-bit)
	extendUnsignedX        registerExtension = 3 // Unsigned Extend X (no-op for 64-bit)
	extendSignedByte       registerExtension = 4 // Signed Extend Byte
	extendSignedHalfword   registerExtension = 5 // Signed Extend Halfword
	extendSignedWord       registerExtension = 6 // Signed Extend Word
	extendSignedX          registerExtension = 7 // Signed Extend X (no-op for 64-bit)
)

// Abs (ABS/FABS) computes the absolute value of the source register and
// stores the result in the destination register.
func Abs[T X[int64] | D[int64] | W[int32] | V[[16]int8] | V[[8]int16] | V[[4]int32] | V[[2]int64]](dst, src T) Instruction {
	switch src := any(src).(type) {
	case X[int64]:
		return 0b101101011000000001<<13 | rd(dst) | rn(src)
	case W[int32]:
		return 0b001101011000000001<<13 | rd(dst) | rn(src)
	case D[int64]:
		return 0b010111101110000010111<<11 | rd(dst) | rn(src)
	}
	return 0b010011100010000010111<<11 | rd(dst) | rn(src) | size(src)<<22
}

// AddWithCarry (ADC) adds two registers and the carry flag, and stores
func AddWithCarry[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst, a, b T) Instruction {
	switch any(dst).(type) {
	case X[int64], X[uint64]:
		return 0b1001101<<25 | rd(dst) | rn(a) | rm(b)
	}
	return 0b0001101<<25 | rd(dst) | rn(a) | rm(b)
}

// AddWithCarrySettingFlags (ADCS) adds two registers and the carry flag, stores
// the result in the destination register, and updates the flags.
func AddWithCarrySettingFlags[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst, a, b T) Instruction {
	switch any(dst).(type) {
	case X[int64], X[uint64]:
		return 0b1011101<<25 | rd(dst) | rn(a) | rm(b)
	}
	return 0b0011101<<25 | rd(dst) | rn(a) | rm(b)
}

// addShifted (ADD) adds two registers, possibly shifted by a constant,
// and stores the result in the destination register.
func addShifted[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst, a, b T, shift shift, amount uint8) Instruction {
	return 0b10001011<<24 | rd(dst) | rn(a) | rm(b) | imm6(amount)<<10 | imm2(shift)<<22
}

// addExtended (ADD) adds two registers, possibly extended by a constant,
// and stores the result in the destination register.
func addExtended[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst, a, b T, extend registerExtension, amount uint8) Instruction {
	return 0b10001011001<<21 | rd(dst) | rn(a) | rm(b) | imm6(amount)<<10 | imm3(extend)<<13
}

// Add (ADD) adds two registers and stores the result in the destination register.
func Add[
	A X[int64] | W[int32] | X[uint64] | W[uint32],
	B Imm12 |
		W[uint8] |
		W[uint16] |
		W[uint32] |
		X[uint64] |
		W[int8] |
		W[int16] |
		W[int32] |
		X[int64],
](dst, a A, b B) Instruction {
	switch val := any(b).(type) {
	case Imm12:
		return 0b10010001<<24 | rd(dst) | rn(a&0x1F)<<5 | imm12(val)<<10
	case X[uint64]:
		return 0b10001011<<24 | rd(dst) | rn(a) | rm(X[uint64](b))
	case W[uint8]:
		return addExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendUnsignedByte, 0)
	case W[uint16]:
		return addExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendUnsignedHalfword, 0)
	case W[uint32]:
		return addExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendUnsignedWord, 0)
	case W[int8]:
		return addExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendSignedByte, 0)
	case W[int16]:
		return addExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendSignedHalfword, 0)
	case W[int32]:
		return addExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendSignedWord, 0)
	case X[int64]:
		return addExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendSignedX, 0)
	default:
		panic("unreachable")
	}
}

// AddToTaggedPointer (ADDG) adds an immediate offset to a tagged pointer. dst receives the result,
// src is the source pointer, offset is the byte offset (0-1008, multiple of 16), tagOffset
// adjusts the tag (-8 to 7).
func AddToTaggedPointer(dst, src X[TaggedPointer], offset Imm6, tagOffset Imm4) Instruction {
	return 0b1001000110<<22 | rd(dst) | rn(src) | imm6(offset)<<16 | imm4(tagOffset)<<10
}

// AddToCheckedPointer (ADDPT) that adds an offset stored in a register to a checked pointer.
// dst receives the result, src is the source pointer, the offset can be shifted by up to 3bits.
func AddToCheckedPointer(dst, ptr X[CheckedPointer], val X[uintptr], shift Imm3) Instruction {
	return 0b1001101<<25 | rd(dst) | rn(ptr) | rm(val) | imm3(shift)<<10
}

// Return (RET) returns from a subroutine.
func Return() Instruction { return 0b1101011001011111<<16 | rn(X[uint64](30)) }
