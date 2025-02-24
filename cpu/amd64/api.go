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

// Add adds the source to the destination.
func Add[
	A AnyRegister,
	B canBeAddedToRegister[A],
](dst A, src B) Appender {
	return add[A, B]{args: xyz.NewPair(dst, src)}
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

// Aaa performs ASCII adjust after addition.
//
//asm:AAA
func Aaa() Appender { return aaa }

// Aas performs ASCII adjust after subtraction.
//
//asm:AAS
func Aas() Appender { return aas }

// Aad performs ASCII adjust after division.
//
//asm:AAD
func Aad(op uint8) Appender { return aad(op) }

// Aam performs ASCII adjust after multiplication.
//
//asm:AAM
func Aam(op uint8) Appender { return aam(op) }



// Mov moves data between registers or memory.
//
//asm:MOV
func Mov[
	A AnyRegister,
	B canBeAddedToRegister[A],
](dst A, src B) Appender {
	return mov[A, B]{args: xyz.NewPair(dst, src)}
}

// Sub subtracts the source from the destination.
//
//asm:SUB
func Sub[
	A AnyRegister,
	B canBeAddedToRegister[A],
](dst A, src B) Appender {
	return sub[A, B]{args: xyz.NewPair(dst, src)}
}

// Cmp compares the source with the destination.
//
//asm:CMP
func Cmp[
	A AnyRegister,
	B canBeAddedToRegister[A],
](dst A, src B) Appender {
	return cmp[A, B]{args: xyz.NewPair(dst, src)}
}

// Test performs a bitwise AND and sets flags.
//
//asm:TEST
func Test[
	A AnyRegister,
	B canBeAddedToRegister[A],
](dst A, src B) Appender {
	return testInstr[A, B]{args: xyz.NewPair(dst, src)}
}

// Push pushes a value onto the stack.
//
//asm:PUSH
func Push[A AnyRegister](r A) Appender {
	return push[A]{r: r}
}

// Pop pops a value from the stack.
//
//asm:POP
func Pop[A AnyRegister](r A) Appender {
	return pop[A]{r: r}
}

// Jump performs an unconditional jump.
//
//asm:JMP
func Jump(disp int32) Appender {
	return jmp{disp: disp}
}

// Call performs a function call.
//
//asm:CALL
func Call(disp int32) Appender {
	return call{disp: disp}
}

// Multiply performs unsigned multiplication.
//
//asm:MUL
func Multiply[A AnyRegister](r A) Appender {
	return mul[A]{r: r}
}

// Divide performs unsigned division.
//
//asm:DIV
func Divide[A AnyRegister](r A) Appender {
	return divInstr[A]{r: r}
}
