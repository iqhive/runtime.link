package amd64

import (
	"encoding"

	"runtime.link/xyz"
)

// AddWithCarry adds the source to the destination with carry.
func AddWithCarry[
	A AnyRegister | AnyPointer,
	B canBeAddedTo[A],
](dst A, src B) ADC[A, B] {
	return ADC[A, B]{Args: xyz.NewPair(dst, src)}
}

// Return returns from the current function.
func Return() encoding.BinaryAppender { return RET }
