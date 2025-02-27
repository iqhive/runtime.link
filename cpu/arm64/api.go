package arm64

// Abs (ABS) computes the absolute value of the source register and
// stores the result in the destination register.
func Abs(dest, src Register) Assembly {
	return instruction{
		op: 0x9B207C00,
		rd: dest,
		rn: src,
	}
}

// Return (RET) returns from a subroutine.
func Return() Assembly { return ret }
