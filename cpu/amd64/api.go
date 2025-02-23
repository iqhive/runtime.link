package amd64

import (
	"runtime.link/xyz"
)

// AddWithCarry adds the source to the destination with carry.
func AddWithCarry[
	A AnyRegister,
	B canBeAddedToRegister[A],
](dst A, src B) Appender {
	return adc[A, B]{args: xyz.NewPair(dst, src)}
}

// MemoryAddWithCarry adds the source to the destination with carry.
func MemoryAddWithCarry[
	A AnyPointer,
	B canBeAddedToPointer[A],
](dst A, src B) Appender {
	return adc[A, B]{args: xyz.NewPair(dst, src)}
}

// Return returns from the current function.
func Return() Appender { return ret }
