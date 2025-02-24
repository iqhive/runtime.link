package amd64

import (
	"runtime.link/xyz"
)

// AddWithCarry (ADC) adds the source to the destination with carry.
func AddWithCarry[A AnyRegister, B canBeAddedToRegister[A]](dst A, src B) Appender {
	return adc[A, B]{args: xyz.NewPair(dst, src)}
}

// Add (ADD) adds the source to the destination.
func Add[A AnyRegister, B canBeAddedToRegister[A]](dst A, src B) Appender {
	return add[A, B]{args: xyz.NewPair(dst, src)}
}

// MemoryAddWithCarry (ADC) adds the source to the destination with carry.
func MemoryAddWithCarry[A AnyPointer, B canBeAddedToPointer[A]](dst A, src B) Appender {
	return adc[A, B]{args: xyz.NewPair(dst, src)}
}

// Return (RET) returns from the current function.
func Return() Appender { return ret }

// BitwiseAnd (AND) performs a bitwise AND operation.
//
//asm:AND
func BitwiseAnd[
	A AnyRegister,
	B canBeAddedToRegister[A],
](dst A, src B) Appender {
	return and[A, B]{args: xyz.NewPair(dst, src)}
}

// BitwiseOr (OR) performs a bitwise OR operation.
//
//asm:OR
func BitwiseOr[
	A AnyRegister,
	B canBeAddedToRegister[A],
](dst A, src B) Appender {
	return or[A, B]{args: xyz.NewPair(dst, src)}
}

// BitwiseXor (XOR) performs a bitwise XOR operation.
//
//asm:XOR
func BitwiseXor[
	A AnyRegister,
	B canBeAddedToRegister[A],
](dst A, src B) Appender {
	return xor[A, B]{args: xyz.NewPair(dst, src)}
}

// MemoryBitwiseAnd (AND) performs a bitwise AND operation with memory operand.
//
//asm:AND
func MemoryBitwiseAnd[A AnyPointer, B canBeAddedToPointer[A]](dst A, src B) Appender {
	return and[A, B]{args: xyz.NewPair(dst, src)}
}

// MemoryBitwiseOr (OR) performs a bitwise OR operation with memory operand.
//
//asm:OR
func MemoryBitwiseOr[A AnyPointer, B canBeAddedToPointer[A]](dst A, src B) Appender {
	return or[A, B]{args: xyz.NewPair(dst, src)}
}

// MemoryBitwiseXor (XOR) performs a bitwise XOR operation with memory operand.
//
//asm:XOR
func MemoryBitwiseXor[A AnyPointer, B canBeAddedToPointer[A]](dst A, src B) Appender {
	return xor[A, B]{args: xyz.NewPair(dst, src)}
}
