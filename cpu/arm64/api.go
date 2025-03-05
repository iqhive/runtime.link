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
// stores the result in the destination register. X/D requires CSSC.
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

// AddWithCarry (ADCS) adds two registers and the carry flag, and stores the result in the destination register.
func AddWithCarry[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst, a, b T, set_flags bool) Instruction {
	if set_flags {
		switch any(dst).(type) {
		case X[int64], X[uint64]:
			return 0b1011101<<25 | rd(dst) | rn(a) | rm(b)
		}
		return 0b0011101<<25 | rd(dst) | rn(a) | rm(b)
	}
	switch any(dst).(type) {
	case X[int64], X[uint64]:
		return 0b1001101<<25 | rd(dst) | rn(a) | rm(b)
	}
	return 0b0001101<<25 | rd(dst) | rn(a) | rm(b)
}

// addShifted (ADD) adds two registers, possibly shifted by a constant,
// and stores the result in the destination register.
func addShifted[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst, a, b T, shift shift, amount Uint6) Instruction {
	return 0b00001011<<24 | sf(dst) | rd(dst) | rn(a) | rm(b) | imm6(amount)<<10 | imm2(shift)<<22
}

// addExtended (ADD) adds two registers, possibly extended by a constant,
// and stores the result in the destination register.
func addExtended[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst, a, b T, extend registerExtension, amount Uint6) Instruction {
	return 0b00001011001<<21 | sf(dst) | rd(dst) | rn(a) | rm(b) | imm6(amount)<<10 | imm3(extend)<<13
}

// Add (ADD/ADDS) adds two registers and stores the result in the destination register.
func Add[
	A X[int64] | W[int32] | X[uint64] | W[uint32],
	B Int12 | Uint12 | W[uint8] | W[uint16] | W[uint32] | X[uint64] | W[int8] | W[int16] | W[int32] | X[int64],
](dst, a A, b B, set_flags bool) Instruction {
	var set_flags_bit Instruction
	if set_flags {
		set_flags_bit = 1 << 29
	}
	switch val := any(b).(type) {
	case Int12:
		return 0b00010001<<24 | sf(dst) | rd(dst) | rn(a&0x1F)<<5 | imm12(val)<<10 | set_flags_bit
	case X[uint64]:
		return 0b00001011<<24 | sf(dst) | rd(dst) | rn(a) | rm(X[uint64](b)) | set_flags_bit
	case W[uint8]:
		return addExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendUnsignedByte, 0) | set_flags_bit
	case W[uint16]:
		return addExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendUnsignedHalfword, 0) | set_flags_bit
	case W[uint32]:
		return addExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendUnsignedWord, 0) | set_flags_bit
	case W[int8]:
		return addExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendSignedByte, 0) | set_flags_bit
	case W[int16]:
		return addExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendSignedHalfword, 0) | set_flags_bit
	case W[int32]:
		return addExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendSignedWord, 0) | set_flags_bit
	case X[int64]:
		return addExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendSignedX, 0) | set_flags_bit
	default:
		panic("unreachable")
	}
}

// AddToTaggedPointer (ADDG) adds an immediate offset to a tagged pointer. dst receives the result,
// src is the source pointer, offset is the byte offset (0-1008, multiple of 16), tagOffset
// adjusts the tag (-8 to 7).
func AddToTaggedPointer(dst, src X[TaggedPointer], offset Uint6, tagOffset Int4) Instruction {
	return 0b1001000110<<22 | rd(dst) | rn(src) | imm6(offset)<<16 | imm4(tagOffset)<<10
}

// AddToCheckedPointer (ADDPT) that adds an offset stored in a register to a checked pointer.
// dst receives the result, src is the source pointer, the offset can be shifted by up to 3bits.
func AddToCheckedPointer(dst, ptr X[CheckedPointer], val X[uintptr], shift Uint3) Instruction {
	return 0b1001101<<25 | rd(dst) | rn(ptr) | rm(val) | imm3(shift)<<10
}

// GetProgramCounterAddress (ADR/ADRP) returns the address of the program counter (PC) offset by the given
// 21bit 'offset'. The result is stored in the destination register.
func GetProgramCounterAddress[T X[uint64] | W[uint32]](dst T, offset Int21, align4KB bool) Instruction {
	var align_bit Instruction
	if align4KB {
		align_bit = 1 << 31
	}
	return 0b0001<<28 | rd(dst) | immhi(offset) | immlo(offset) | align_bit
}

// subShifted (SUB) subtracts two registers, possibly shifted by a constant,
// and stores the result in the destination register.
func subShifted[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst, a, b T, shift shift, amount Uint6) Instruction {
	return 0b01001011<<24 | sf(dst) | rd(dst) | rn(a) | rm(b) | imm6(amount)<<10 | imm2(shift)<<22
}

// subExtended (SUB) subtracts two registers, possibly extended by a constant,
// and stores the result in the destination register.
func subExtended[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst, a, b T, extend registerExtension, amount Uint6) Instruction {
	return 0b01001011001<<21 | sf(dst) | rd(dst) | rn(a) | rm(b) | imm6(amount)<<10 | imm3(extend)<<13
}

// Sub (SUB) subtracts the second register from the first register and stores the result in the destination register.
func Sub[
	A X[int64] | W[int32] | X[uint64] | W[uint32],
	B Int12 | Uint12 | W[uint8] | W[uint16] | W[uint32] | X[uint64] | W[int8] | W[int16] | W[int32] | X[int64],
](dst, a A, b B, set_flags bool) Instruction {
	var set_flags_bit Instruction
	if set_flags {
		set_flags_bit = 1 << 29
	}
	switch val := any(b).(type) {
	case Int12:
		return 0b01010001<<24 | sf(dst) | rd(dst) | rn(a&0x1F)<<5 | imm12(val)<<10 | set_flags_bit
	case X[uint64]:
		return 0b01001011<<24 | sf(dst) | rd(dst) | rn(a) | rm(X[uint64](b)) | set_flags_bit
	case W[uint8]:
		return subExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendUnsignedByte, 0) | set_flags_bit
	case W[uint16]:
		return subExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendUnsignedHalfword, 0) | set_flags_bit
	case W[uint32]:
		return subExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendUnsignedWord, 0) | set_flags_bit
	case W[int8]:
		return subExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendSignedByte, 0) | set_flags_bit
	case W[int16]:
		return subExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendSignedHalfword, 0) | set_flags_bit
	case W[int32]:
		return subExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendSignedWord, 0) | set_flags_bit
	case X[int64]:
		return subExtended(X[uint64](dst), X[uint64](a), X[uint64](b), extendSignedX, 0) | set_flags_bit
	default:
		panic("unreachable")
	}
}

// BitwiseAnd (AND) performs a bitwise AND operation on two registers and stores the result in the destination register.
func BitwiseAnd[
	A X[int64] | W[int32] | X[uint64] | W[uint32],
	B Int12 | Uint12 | X[uint64] | W[uint32] | X[int64] | W[int32],
](dst, a A, b B, set_flags bool) Instruction {
	var set_flags_bit Instruction
	if set_flags {
		set_flags_bit = 1 << 29
	}
	switch val := any(b).(type) {
	case Int12:
		return 0b00010010<<24 | sf(dst) | rd(dst) | rn(a&0x1F)<<5 | imm12(val)<<10 | set_flags_bit
	case X[uint64], X[int64]:
		return 0b00001010<<24 | sf(dst) | rd(dst) | rn(a) | rm(X[uint64](b)) | set_flags_bit
	case W[uint32], W[int32]:
		return 0b00001010<<24 | sf(dst) | rd(dst) | rn(a) | rm(W[uint32](b)) | set_flags_bit
	default:
		panic("unreachable")
	}
}

// BitwiseOr (ORR) performs a bitwise OR operation on two registers and stores the result in the destination register.
func BitwiseOr[
	A X[int64] | W[int32] | X[uint64] | W[uint32],
	B Int12 | Uint12 | X[uint64] | W[uint32] | X[int64] | W[int32],
](dst, a A, b B) Instruction {
	switch val := any(b).(type) {
	case Int12:
		return 0b00110010<<24 | sf(dst) | rd(dst) | rn(a&0x1F)<<5 | imm12(val)<<10
	case X[uint64], X[int64]:
		return 0b00101010<<24 | sf(dst) | rd(dst) | rn(a) | rm(X[uint64](b))
	case W[uint32], W[int32]:
		return 0b00101010<<24 | sf(dst) | rd(dst) | rn(a) | rm(W[uint32](b))
	default:
		panic("unreachable")
	}
}

// BitwiseXor (EOR) performs a bitwise exclusive OR operation on two registers and stores the result in the destination register.
func BitwiseXor[
	A X[int64] | W[int32] | X[uint64] | W[uint32],
	B Int12 | Uint12 | X[uint64] | W[uint32] | X[int64] | W[int32],
](dst, a A, b B) Instruction {
	switch val := any(b).(type) {
	case Int12:
		return 0b01010010<<24 | sf(dst) | rd(dst) | rn(a&0x1F)<<5 | imm12(val)<<10
	case X[uint64], X[int64]:
		return 0b01001010<<24 | sf(dst) | rd(dst) | rn(a) | rm(X[uint64](b))
	case W[uint32], W[int32]:
		return 0b01001010<<24 | sf(dst) | rd(dst) | rn(a) | rm(W[uint32](b))
	default:
		panic("unreachable")
	}
}

// Compare (SUBS) compares two registers by subtracting the second from the first and setting the condition flags.
func Compare[
	A X[int64] | W[int32] | X[uint64] | W[uint32],
	B Int12 | Uint12 | W[uint8] | W[uint16] | W[uint32] | X[uint64] | W[int8] | W[int16] | W[int32] | X[int64],
](a A, b B) Instruction {
	// CMP is equivalent to SUBS with XZR/WZR as the destination
	switch any(a).(type) {
	case X[int64], X[uint64]:
		return Sub(X[int64](31), X[int64](a), b, true)
	default:
		return Sub(W[int32](31), W[int32](a), b, true)
	}
}

// CompareNegative (ADDS) compares two registers by adding them and setting the condition flags.
func CompareNegative[
	A X[int64] | W[int32] | X[uint64] | W[uint32],
	B Int12 | Uint12 | W[uint8] | W[uint16] | W[uint32] | X[uint64] | W[int8] | W[int16] | W[int32] | X[int64],
](a A, b B) Instruction {
	// CMN is equivalent to ADDS with XZR/WZR as the destination
	switch any(a).(type) {
	case X[int64], X[uint64]:
		return Add(X[int64](31), X[int64](a), b, true)
	default:
		return Add(W[int32](31), W[int32](a), b, true)
	}
}

// MoveWideImmediate (MOVZ) moves a 16-bit immediate value to a register, with optional left shift.
func MoveWideImmediate[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst T, imm Uint16, shift Uint2) Instruction {
	return 0b10010010100<<20 | sf(dst) | rd(dst) | imm16(imm)<<5 | imm2(shift)<<21
}

// MoveWideImmediateNot (MOVN) moves the bitwise NOT of a 16-bit immediate value to a register, with optional left shift.
func MoveWideImmediateNot[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst T, imm Uint16, shift Uint2) Instruction {
	return 0b10010010000<<20 | sf(dst) | rd(dst) | imm16(imm)<<5 | imm2(shift)<<21
}

// MoveWideImmediateKeep (MOVK) moves a 16-bit immediate value to a register, keeping other bits unchanged, with optional left shift.
func MoveWideImmediateKeep[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst T, imm Uint16, shift Uint2) Instruction {
	return 0b10010010110<<20 | sf(dst) | rd(dst) | imm16(imm)<<5 | imm2(shift)<<21
}

// BitfieldMove (BFM) moves a bitfield from one register to another.
func BitfieldMove[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst, src T, immr, imms Uint6) Instruction {
	return 0b10010011<<24 | sf(dst) | rd(dst) | rn(src) | imm6(immr)<<16 | imm6(imms)<<10 | 1<<22
}

// SignedBitfieldMove (SBFM) moves a signed bitfield from one register to another.
func SignedBitfieldMove[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst, src T, immr, imms Uint6) Instruction {
	return 0b10010011<<24 | sf(dst) | rd(dst) | rn(src) | imm6(immr)<<16 | imm6(imms)<<10 | 0<<22
}

// UnsignedBitfieldMove (UBFM) moves an unsigned bitfield from one register to another.
func UnsignedBitfieldMove[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst, src T, immr, imms Uint6) Instruction {
	return 0b10010011<<24 | sf(dst) | rd(dst) | rn(src) | imm6(immr)<<16 | imm6(imms)<<10 | 2<<22
}

// Extract (EXTR) extracts a register from a pair of registers.
func Extract[T X[int64] | W[int32] | X[uint64] | W[uint32]](dst, src1, src2 T, lsb Uint6) Instruction {
	return 0b00010011100<<21 | sf(dst) | rd(dst) | rn(src1) | rm(src2) | imm6(lsb)<<10
}

// Return (RET) returns from a subroutine.
func Return() Instruction { return 0b1101011001011111<<16 | rn(X[uint64](30)) }
