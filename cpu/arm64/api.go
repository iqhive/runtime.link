package arm64

// ShiftType for shifted register form
type ShiftType uint8

const (
	LogicalShiftLeft     ShiftType = 0 // Logical Shift Left
	LogicalShiftRight    ShiftType = 1 // Logical Shift Right
	ArithmeticShiftRight ShiftType = 2 // Arithmetic Shift Right
)

// ExtendType for extended register form
type ExtendType uint8

const (
	UnsignedExtendByte     ExtendType = 0 // Unsigned Extend Byte
	UnsignedExtendHalfword ExtendType = 1 // Unsigned Extend Halfword
	UnsignedExtendWord     ExtendType = 2 // Unsigned Extend Word (also LSL for 32-bit)
	UnsignedExtendX        ExtendType = 3 // Unsigned Extend X (no-op for 64-bit)
	SignedExtendByte       ExtendType = 4 // Signed Extend Byte
	SignedExtendHalfword   ExtendType = 5 // Signed Extend Halfword
	SignedExtendWord       ExtendType = 6 // Signed Extend Word
	SignedExtendX          ExtendType = 7 // Signed Extend X (no-op for 64-bit)
)

// Abs (ABS) computes the absolute value of the source register and
// stores the result in the destination register.
func Abs(dest, src Register) Assembly {
	return instruction{
		op: 0x9B207C00,
		rd: dest,
		rn: src,
	}
}

// AddWithCarry (ADC) adds two registers and the carry flag, and stores
func AddWithCarry(dest, a, b Register) Assembly {
	return instruction{
		op: 0x9A000000,
		rd: dest,
		rn: a,
		rm: b,
	}
}

// AddShifted (ADD) adds two registers, possibly shifted by a constant,
// and stores the result in the destination register.
func AddShifted(dst, a, b Register, shift ShiftType, amount uint8) Assembly {
	if amount > 63 {
		amount = 63 // Clamp to max 64-bit shift
	}
	instruction := uint32(0x8B000000)        // 64-bit ADD (shifted)
	instruction |= uint32(dst & 0x1F)        // Rd: bits 4-0
	instruction |= uint32(a&0x1F) << 5       // Rn: bits 9-5
	instruction |= uint32(b&0x1F) << 16      // Rm: bits 20-16
	instruction |= uint32(amount&0x3F) << 10 // imm6: bits 15-10
	instruction |= uint32(shift&0x3) << 22   // shift: bits 23-22
	return literal(instruction)
}

// AddExtended (ADD) adds two registers, possibly extended by a constant,
// and stores the result in the destination register.
func AddExtended(dst, a, b Register, extend ExtendType, amount uint8) Assembly {
	if amount > 63 {
		amount = 63 // Clamp to max 64-bit shift
	}
	instruction := uint32(0x8B200000)        // 64-bit ADD (extended)
	instruction |= uint32(dst & 0x1F)        // Rd: bits 4-0
	instruction |= uint32(a&0x1F) << 5       // Rn: bits 9-5
	instruction |= uint32(b&0x1F) << 16      // Rm: bits 20-16
	instruction |= uint32(amount&0x3F) << 10 // imm6: bits 15-10
	instruction |= uint32(extend&0x7) << 13  // extend: bits 15-13
	return literal(instruction)
}

// Add (ADD) adds two registers and stores the result in the destination register.
func Add(dest, a, b Register) Assembly {
	return instruction{
		op: 0x8B000000,
		rd: dest,
		rn: a,
		rm: b,
	}
}

// Return (RET) returns from a subroutine.
func Return() Assembly { return ret }
