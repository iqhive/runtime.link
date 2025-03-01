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
func Abs[T X | D | anyVector](dst, src T) Instruction {
	switch src := any(src).(type) {
	case X:
		return 0b101101011000000001<<13 | rd(dst) | rn(src)
	case D:
		return 0b010111101110000010111<<11 | rd(dst) | rn(src)
	}
	return 0b010011100010000010111<<11 | rd(dst) | rn(src) | size(src)<<22
}

// AddWithCarry (ADC) adds two registers and the carry flag, and stores
func AddWithCarry(dst, a, b X) Instruction {
	return 0b1001101<<25 | rd(dst) | rn(a) | rm(b)
}

// AddWithCarrySetFlags (ADCS) adds two registers and the carry flag, stores
// the result in the destination register, and updates the flags.
func AddWithCarrySetFlags(dst, a, b X) Instruction {
	return 0b1011101<<25 | rd(dst) | rn(a) | rm(b)
}

// addShifted (ADD) adds two registers, possibly shifted by a constant,
// and stores the result in the destination register.
func addShifted(dst, a, b X, shift shift, amount uint8) Instruction {
	return 0b10001011<<24 | rd(dst) | rn(a) | rm(b) | imm6(amount)<<10 | imm2(shift)<<22
}

// addExtended (ADD) adds two registers, possibly extended by a constant,
// and stores the result in the destination register.
func addExtended(dst, a, b X, extend registerExtension, amount uint8) Instruction {
	return 0b10001011001<<21 | rd(dst) | rn(a) | rm(b) | imm6(amount)<<10 | imm3(extend)<<13
}

// Add (ADD) adds two registers and stores the result in the destination register.
func Add[
	T Imm12 |
		Reg[uint8] |
		Reg[uint16] |
		Reg[uint32] |
		Reg[uint64] |
		Reg[int8] |
		Reg[int16] |
		Reg[int32] |
		Reg[int64],
](dst, a X, b T) Instruction {
	switch b := any(b).(type) {
	case Imm12:
		return 0b10010001<<24 | rd(dst) | rn(a&0x1F)<<5 | imm12(b)<<10
	case X:
		return 0b10001011<<24 | rd(dst) | rn(a) | rm(b)
	case Reg[uint8]:
		return addExtended(dst, a, X(b), extendUnsignedByte, 0)
	case Reg[uint16]:
		return addExtended(dst, a, X(b), extendUnsignedHalfword, 0)
	case Reg[uint32]:
		return addExtended(dst, a, X(b), extendUnsignedWord, 0)
	case Reg[int8]:
		return addExtended(dst, a, X(b), extendSignedByte, 0)
	case Reg[int16]:
		return addExtended(dst, a, X(b), extendSignedHalfword, 0)
	case Reg[int32]:
		return addExtended(dst, a, X(b), extendSignedWord, 0)
	case Reg[int64]:
		return addExtended(dst, a, X(b), extendSignedX, 0)
	default:
		panic("unreachable")
	}
}

// AddToTaggedPointer (ADDG) adds an offset to a tagged pointer. dst receives the result, src is the source pointer, offset
// is the byte offset (0-1008, multiple of 16), tagOffset adjusts the tag (-8 to 7).
func AddToTaggedPointer(dst Reg[TaggedPointer], src X, offset Imm6, tagOffset Imm4) Instruction {
	return 0b1001000110<<22 | rd(dst) | rn(src) | imm6(offset)<<16 | imm4(tagOffset)<<10
}

// AddToCheckedPointer creates an ADDPT instruction (64-bit) that adds an immediate to a checked pointer.
// dst receives the result, src is the source pointer, imm is the offset (0-4095, multiple of 8).
func AddToCheckedPointer(dst Reg[CheckedPointer], src X, shift Imm3) Instruction {
	return 0b1001101<<25 | rd(dst) | rn(src) | imm3(shift)<<10
}

// Return (RET) returns from a subroutine.
func Return() Instruction { return 0b1101011001011111<<16 | rn(X(30)) }
