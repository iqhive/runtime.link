package amd64

import (
	"fmt"

	"runtime.link/xyz"
)

type Appender interface {
	AppendAMD64(b []byte) []byte
}

type literal string

const (
	aaa literal = "\x37" // ASCII Adjust After Addition
	aas literal = "\x3F" // ASCII Adjust AL After Subtraction
	ret literal = "\xC3" // Return
)

func (l literal) AppendAMD64(b []byte) []byte { return append(b, l...) }

type (
	aad uint8 // ASCII Adjust AX Before Division
	aam uint8 // ASCII Adjust AX After Multiply
)

func (op aad) AppendAMD64(b []byte) []byte { return append(b, 0xD5, byte(op)) }
func (op aam) AppendAMD64(b []byte) []byte { return append(b, 0xD4, byte(op)) }

type adc[A, B any] struct {
	args xyz.Pair[A, B]
}

func (op adc[A, B]) AppendAMD64(b []byte) []byte {
	switch args := any(op.args).(type) {
	// Register-to-Register
	case xyz.Pair[Register[uint8], Register[uint8]]:
		dst, src := args.Split()
		return append(b, 0x10, 0xC0|byte(dst)<<3|byte(src))
	case xyz.Pair[Register[uint16], Register[uint16]]:
		dst, src := args.Split()
		return append(b, 0x66, 0x11, 0xC0|byte(dst)<<3|byte(src))
	case xyz.Pair[Register[uint32], Register[uint32]]:
		dst, src := args.Split()
		return append(b, 0x11, 0xC0|byte(dst)<<3|byte(src))
	case xyz.Pair[Register[uint64], Register[uint64]]:
		dst, src := args.Split()
		rex := byte(0x48) // REX.W for 64-bit
		if dst >= 8 {
			rex |= 0x04 // REX.R for dst
		}
		if src >= 8 {
			rex |= 0x01 // REX.B for src
		}
		return append(b, rex, 0x11, 0xC0|byte(dst&0x07)<<3|byte(src&0x07))
	// Register-to-Immediate
	case xyz.Pair[Register[uint8], Imm8]:
		dst, src := args.Split()
		if dst == AL {
			return append(b, 0x14, byte(src))
		}
		return append(b, 0x80, 0xD0|byte(dst), byte(src))
	case xyz.Pair[Register[uint16], Imm8]:
		dst, src := args.Split()
		return append(b, 0x66, 0x83, 0xD0|byte(dst), byte(src)) // Sign-extended imm8
	case xyz.Pair[Register[uint16], Imm16]:
		dst, src := args.Split()
		if dst == AX {
			return append(b, 0x66, 0x15, byte(src), byte(src>>8))
		}
		return append(b, 0x66, 0x81, 0xD0|byte(dst), byte(src), byte(src>>8))
	case xyz.Pair[Register[uint32], Imm8]:
		dst, src := args.Split()
		return append(b, 0x83, 0xD0|byte(dst), byte(src))
	case xyz.Pair[Register[uint32], Imm16]:
		dst, src := args.Split()
		return append(b, 0x81, 0xD0|byte(dst), byte(src), byte(src>>8))
	case xyz.Pair[Register[uint32], Imm32]:
		dst, src := args.Split()
		if dst == EAX {
			return append(b, 0x05, byte(src), byte(src>>8), byte(src>>16), byte(src>>24))
		}
		return append(b, 0x81, 0xD0|byte(dst), byte(src), byte(src>>8), byte(src>>16), byte(src>>24))
	// Register-to-Memory
	case xyz.Pair[Register[uint8], Pointer[uint8]]:
		dst, src := args.Split()
		return append(b, 0x12, 0x00|byte(dst)<<3|byte(src))
	case xyz.Pair[Register[uint16], Pointer[uint16]]:
		dst, src := args.Split()
		return append(b, 0x66, 0x13, 0x00|byte(dst)<<3|byte(src))
	case xyz.Pair[Register[uint32], Pointer[uint32]]:
		dst, src := args.Split()
		return append(b, 0x13, 0x00|byte(dst)<<3|byte(src))
	case xyz.Pair[Register[uint64], Pointer[uint64]]:
		dst, src := args.Split()
		rex := byte(0x48) // REX.W
		if dst >= 8 {
			rex |= 0x04 // REX.R
		}
		if src >= 8 {
			rex |= 0x01 // REX.B
		}
		return append(b, rex, 0x13, 0x00|byte(dst&0x07)<<3|byte(src&0x07))
	// Memory-to-Register
	case xyz.Pair[Pointer[uint8], Register[uint8]]:
		dst, src := args.Split()
		return append(b, 0x10, 0x00|byte(src)<<3|byte(dst&0x07))
	case xyz.Pair[Pointer[uint16], Register[uint16]]:
		dst, src := args.Split()
		return append(b, 0x66, 0x11, 0x00|byte(src)<<3|byte(dst&0x07))
	case xyz.Pair[Pointer[uint32], Register[uint32]]:
		dst, src := args.Split()
		return append(b, 0x11, 0x00|byte(src)<<3|byte(dst&0x07))
	case xyz.Pair[Pointer[uint64], Register[uint64]]:
		dst, src := args.Split()
		rex := byte(0x48) // REX.W for 64-bit
		if src >= 8 {
			rex |= 0x04 // REX.R for extended source
		}
		if dst >= 8 {
			rex |= 0x01 // REX.B for extended base
		}
		return append(b, rex, 0x11, 0x00|byte(src&0x07)<<3|byte(dst&0x07))

	// Memory-to-Immediate
	case xyz.Pair[Pointer[uint8], Imm8]:
		dst, src := args.Split()
		rex := byte(0)
		if dst >= 8 {
			rex = 0x41 // REX.B for extended base
		}
		if rex != 0 {
			b = append(b, rex)
		}
		return append(b, 0x80, 0x10|byte(dst&0x07), byte(src))
	case xyz.Pair[Pointer[uint16], uint16]: // Assuming Imm16 is uint16
		dst, src := args.Split()
		rex := byte(0)
		if dst >= 8 {
			rex = 0x41 // REX.B
		}
		if rex != 0 {
			b = append(b, rex)
		}
		return append(b, 0x81, 0x10|byte(dst&0x07), uint8(src), uint8(src>>8))
	case xyz.Pair[Pointer[uint32], Imm32]:
		dst, src := args.Split()
		rex := byte(0x48) // REX.W for 64-bit addressing
		if dst >= 8 {
			rex |= 0x01 // REX.B for R8–R15
		}
		return append(b, rex, 0x81, 0x10|byte(dst&0x07),
			uint8(src), uint8(src>>8), uint8(src>>16), uint8(src>>24))
	default:
		panic(fmt.Sprintf("ADC: unsupported operands: %T", op.args))
	}
}

type add[A, B any] struct {
	args xyz.Pair[A, B]
}

func (op add[A, B]) AppendAMD64(b []byte) []byte {
	switch args := any(op.args).(type) {
	// Register-to-Register
	case xyz.Pair[Register[uint8], Register[uint8]]:
		dst, src := args.Split()
		return append(b, 0x00, 0xC0|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint16], Register[uint16]]:
		dst, src := args.Split()
		return append(b, 0x66, 0x01, 0xC0|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint32], Register[uint32]]:
		dst, src := args.Split()
		return append(b, 0x01, 0xC0|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint64], Register[uint64]]:
		dst, src := args.Split()
		rex := byte(0x48) // REX.W for 64-bit
		if dst >= 8 {
			rex |= 0x04 // REX.R for dst
		}
		if src >= 8 {
			rex |= 0x01 // REX.B for src
		}
		return append(b, rex, 0x01, 0xC0|byte(src&0x07)<<3|byte(dst&0x07))

	// Register-to-Immediate
	case xyz.Pair[Register[uint8], Imm8]:
		dst, src := args.Split()
		if dst == AL {
			return append(b, 0x04, byte(src))
		}
		return append(b, 0x80, 0xC0|byte(dst), byte(src))
	case xyz.Pair[Register[uint16], Imm8]:
		dst, src := args.Split()
		return append(b, 0x66, 0x83, 0xC0|byte(dst), byte(src))
	case xyz.Pair[Register[uint16], Imm16]:
		dst, src := args.Split()
		if dst == AX {
			return append(b, 0x66, 0x05, byte(src), byte(src>>8))
		}
		return append(b, 0x66, 0x81, 0xC0|byte(dst), byte(src), byte(src>>8))
	case xyz.Pair[Register[uint32], Imm8]:
		dst, src := args.Split()
		return append(b, 0x83, 0xC0|byte(dst), byte(src))
	case xyz.Pair[Register[uint32], Imm16]:
		dst, src := args.Split()
		return append(b, 0x81, 0xC0|byte(dst), byte(src), byte(src>>8))
	case xyz.Pair[Register[uint32], Imm32]:
		dst, src := args.Split()
		if dst == EAX {
			return append(b, 0x05, byte(src), byte(src>>8), byte(src>>16), byte(src>>24))
		}
		return append(b, 0x81, 0xC0|byte(dst), byte(src), byte(src>>8), byte(src>>16), byte(src>>24))

	// Register-to-Memory
	case xyz.Pair[Register[uint8], Pointer[uint8]]:
		dst, src := args.Split()
		return append(b, 0x00, 0x00|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint16], Pointer[uint16]]:
		dst, src := args.Split()
		return append(b, 0x66, 0x01, 0x00|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint32], Pointer[uint32]]:
		dst, src := args.Split()
		return append(b, 0x01, 0x00|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint64], Pointer[uint64]]:
		dst, src := args.Split()
		rex := byte(0x48)
		if dst >= 8 {
			rex |= 0x04
		}
		if src >= 8 {
			rex |= 0x01
		}
		return append(b, rex, 0x01, 0x00|byte(src&0x07)<<3|byte(dst&0x07))

	// Memory-to-Register
	case xyz.Pair[Pointer[uint8], Register[uint8]]:
		dst, src := args.Split()
		return append(b, 0x02, 0x00|byte(src)<<3|byte(dst&0x07))
	case xyz.Pair[Pointer[uint16], Register[uint16]]:
		dst, src := args.Split()
		return append(b, 0x66, 0x03, 0x00|byte(src)<<3|byte(dst&0x07))
	case xyz.Pair[Pointer[uint32], Register[uint32]]:
		dst, src := args.Split()
		return append(b, 0x03, 0x00|byte(src)<<3|byte(dst&0x07))
	case xyz.Pair[Pointer[uint64], Register[uint64]]:
		dst, src := args.Split()
		rex := byte(0x48)
		if src >= 8 {
			rex |= 0x04
		}
		if dst >= 8 {
			rex |= 0x01
		}
		return append(b, rex, 0x03, 0x00|byte(src&0x07)<<3|byte(dst&0x07))

	// Memory-to-Immediate
	case xyz.Pair[Pointer[uint8], Imm8]:
		dst, src := args.Split()
		rex := byte(0)
		if dst >= 8 {
			rex = 0x41
		}
		if rex != 0 {
			b = append(b, rex)
		}
		return append(b, 0x80, 0x00|byte(dst&0x07), byte(src))
	case xyz.Pair[Pointer[uint16], uint16]:
		dst, src := args.Split()
		rex := byte(0)
		if dst >= 8 {
			rex = 0x41
		}
		if rex != 0 {
			b = append(b, rex)
		}
		return append(b, 0x81, 0x00|byte(dst&0x07), uint8(src), uint8(src>>8))
	case xyz.Pair[Pointer[uint32], Imm32]:
		dst, src := args.Split()
		rex := byte(0x48)
		if dst >= 8 {
			rex |= 0x01
		}
		return append(b, rex, 0x81, 0x00|byte(dst&0x07),
			uint8(src), uint8(src>>8), uint8(src>>16), uint8(src>>24))
	default:
		panic(fmt.Sprintf("ADD: unsupported operands: %T", op.args))
	}
}

// -- MOV ---------------------------------------------------------------
// mov r/m8, r8  => 0x88 /r
// mov r/m16, r16 => [66] 0x89 /r
// mov r/m32, r32 => 0x89 /r
// mov r/m64, r64 => 0x89 /r with REX.W
// mov r8, r/m8  => 0x8A /r
// mov r16, r/m16 => [66] 0x8B /r
// mov r32, r/m32 => 0x8B /r
// mov r64, r/m64 => 0x8B /r with REX.W
type mov[A, B any] struct {
	args xyz.Pair[A, B]
}

func (op mov[A, B]) AppendAMD64(b []byte) []byte {
	switch args := any(op.args).(type) {
	// Register -> Register
	case xyz.Pair[Register[uint8], Register[uint8]]:
		dst, src := args.Split()
		// mov r8, r8 => 0x8A with /r reversed in the modrm
		// or 0x88 with /r if you want src->dst.
		// Decide direction carefully: let's do "dst <- src" → 0x8A
		return append(b, 0x8A, 0xC0|byte(dst)<<3|byte(src))

	case xyz.Pair[Register[uint16], Register[uint16]]:
		dst, src := args.Split()
		// mov r16, r16 => 0x66, 0x8B, modrm
		return append(b, 0x66, 0x8B, 0xC0|byte(dst)<<3|byte(src))

	case xyz.Pair[Register[uint32], Register[uint32]]:
		dst, src := args.Split()
		// mov r32, r32 => 0x8B, modrm
		return append(b, 0x8B, 0xC0|byte(dst)<<3|byte(src))

	case xyz.Pair[Register[uint64], Register[uint64]]:
		dst, src := args.Split()
		// REX.W + mov r64, r64 => 0x48, 0x8B, modrm
		rex := byte(0x48)
		if dst >= 8 {
			rex |= 0x04 // REX.R
		}
		if src >= 8 {
			rex |= 0x01 // REX.B
		}
		return append(b, rex, 0x8B, 0xC0|byte(dst&0x07)<<3|byte(src&0x07))

	// Register -> Memory
	case xyz.Pair[Pointer[uint8], Register[uint8]]:
		dst, src := args.Split()
		// mov r/m8, r8 => 0x88 /r
		return append(b, 0x88, 0x00|byte(src)<<3|byte(dst&0x07))

	// Memory -> Register
	case xyz.Pair[Register[uint8], Pointer[uint8]]:
		dst, src := args.Split()
		// mov r8, r/m8 => 0x8A /r
		return append(b, 0x8A, 0x00|byte(dst)<<3|byte(src&0x07))

	// You can add more cases if desired...

	default:
		panic(fmt.Sprintf("MOV: unsupported operands %T", op.args))
	}
}

// -- SUB ---------------------------------------------------------------
// sub r/m8, r8 => 0x28 /r
// sub r/m16, r16 => [66] 0x29 /r
// sub r/m32, r32 => 0x29 /r
// sub r/m64, r64 => REX.W + 0x29 /r
// sub r8, r/m8 => 0x2A /r
// sub r16, r/m16 => [66] 0x2B /r
// sub r32, r/m32 => 0x2B /r
// sub r64, r/m64 => REX.W + 0x2B /r
// sub al, imm8 => 0x2C ib
// sub eax, imm32 => 0x2D id
// sub r/m8, imm8 => 0x80 /5 ib
type sub[A, B any] struct {
	args xyz.Pair[A, B]
}

func (op sub[A, B]) AppendAMD64(b []byte) []byte {
	switch args := any(op.args).(type) {
	case xyz.Pair[Register[uint8], Register[uint8]]:
		dst, src := args.Split()
		// sub r8, r8 => 0x2A /r (dst <- dst - src)
		return append(b, 0x2A, 0xC0|byte(dst)<<3|byte(src))

	case xyz.Pair[Register[uint64], Register[uint64]]:
		// sub r64, r64 => 0x48, 0x2B /r
		dst, src := args.Split()
		rex := byte(0x48)
		if dst >= 8 {
			rex |= 0x04
		}
		if src >= 8 {
			rex |= 0x01
		}
		return append(b, rex, 0x2B, 0xC0|byte(dst&0x07)<<3|byte(src&0x07))

	// Example: sub AL, imm8 => 0x2C ib
	case xyz.Pair[Register[uint8], Imm8]:
		dst, src := args.Split()
		if dst == AL {
			return append(b, 0x2C, byte(src))
		}
		// sub r/m8, imm8 => 0x80, /5
		return append(b, 0x80, 0xE8|byte(dst), byte(src)) // /5 = 101b

	// Add more coverage as needed...

	default:
		panic(fmt.Sprintf("SUB: unsupported operands: %T", op.args))
	}
}

// -- CMP ---------------------------------------------------------------
// cmp r/m8, r8 => 0x38 /r
// cmp r/m16, r16 => [66] 0x39 /r
// cmp r/m32, r32 => 0x39 /r
// cmp r/m64, r64 => [REX.W] 0x39 /r
// cmp r8, r/m8 => 0x3A /r
// cmp r16, r/m16 => [66] 0x3B /r
// cmp r32, r/m32 => 0x3B /r
// cmp r64, r/m64 => [REX.W] 0x3B /r
// cmp al, imm8 => 0x3C ib
// cmp eax, imm32 => 0x3D id
type cmp[A, B any] struct {
	args xyz.Pair[A, B]
}

func (op cmp[A, B]) AppendAMD64(b []byte) []byte {
	switch args := any(op.args).(type) {
	// cmp r8, r8 => 0x3A /r
	case xyz.Pair[Register[uint8], Register[uint8]]:
		dst, src := args.Split()
		return append(b, 0x3A, 0xC0|byte(dst)<<3|byte(src))

	// cmp al, imm8 => 0x3C ib
	case xyz.Pair[Register[uint8], Imm8]:
		dst, src := args.Split()
		if dst == AL {
			return append(b, 0x3C, byte(src))
		}
		// cmp r/m8, imm8 => 0x80 /7 ib
		return append(b, 0x80, 0xF8|byte(dst), byte(src))

	// Extend for 16/32/64-bit, memory, etc.

	default:
		panic(fmt.Sprintf("CMP: unsupported operands: %T", op.args))
	}
}

// -- TEST --------------------------------------------------------------
// test r/m8, r8 => 0x84 /r
// test r/m16, r16 => [66] 0x85 /r
// test r/m32, r32 => 0x85 /r
// test r/m64, r64 => REX.W + 0x85 /r
// test al, imm8 => 0xA8 ib
// test eax, imm32 => 0xA9 id
type testInstr[A, B any] struct {
	args xyz.Pair[A, B]
}

func (op testInstr[A, B]) AppendAMD64(b []byte) []byte {
	switch args := any(op.args).(type) {
	// test al, imm8 => 0xA8 ib
	case xyz.Pair[Register[uint8], Imm8]:
		reg, imm := args.Split()
		if reg == AL {
			return append(b, 0xA8, byte(imm))
		}
		// test r/m8, imm8 => 0xF6 /0 ib
		return append(b, 0xF6, 0xC0|byte(reg), byte(imm))

	// test r8, r8 => 0x84 /r
	case xyz.Pair[Register[uint8], Register[uint8]]:
		dst, src := args.Split()
		// test r/m8, r8 => 0x84 /r
		return append(b, 0x84, 0xC0|byte(dst)<<3|byte(src))

	default:
		panic(fmt.Sprintf("TEST: unsupported operand combination: %T", op.args))
	}
}

// -- PUSH --------------------------------------------------------------
// push r16 => [66] 0x50 + rw
// push r32 => 0x50 + rd
// push r64 => [REX.W +] 0x50 + rd
type push[A any] struct {
	r A
}

func (op push[A]) AppendAMD64(b []byte) []byte {
	switch r := any(op.r).(type) {
	case Register[uint64]:
		// push r64 => REX.W + (0x50 + regIndex)
		rex := byte(0x48)
		opcode := byte(0x50 | (r & 7))
		if r >= 8 {
			rex |= 0x01
		}
		return append(b, rex, opcode)

	case Register[uint32]:
		// push r32 => 0x50 + regIndex
		opcode := byte(0x50 | (r & 7))
		return append(b, opcode)

	case Register[uint16]:
		// push r16 => 0x66, 0x50 + regIndex
		opcode := byte(0x50 | (r & 7))
		return append(b, 0x66, opcode)

	default:
		panic(fmt.Sprintf("PUSH: unsupported register type %T", op.r))
	}
}

// -- POP ---------------------------------------------------------------
// pop r16 => [66] 0x58 + rw
// pop r32 => 0x58 + rd
// pop r64 => [REX.W +] 0x58 + rd
type pop[A any] struct {
	r A
}

func (op pop[A]) AppendAMD64(b []byte) []byte {
	switch r := any(op.r).(type) {
	case Register[uint64]:
		// pop r64 => REX.W + (0x58 + regIndex)
		rex := byte(0x48)
		opcode := byte(0x58 | (r & 7))
		if r >= 8 {
			rex |= 0x01
		}
		return append(b, rex, opcode)

	case Register[uint32]:
		// pop r32 => 0x58 + regIndex
		opcode := byte(0x58 | (r & 7))
		return append(b, opcode)

	case Register[uint16]:
		// pop r16 => 0x66, 0x58 + regIndex
		opcode := byte(0x58 | (r & 7))
		return append(b, 0x66, opcode)

	default:
		panic(fmt.Sprintf("POP: unsupported register type %T", op.r))
	}
}

// -- JMP ---------------------------------------------------------------
// jmp rel8 => 0xEB, disp8
// jmp rel32 => 0xE9, disp32
type jmp struct {
	// For simplicity, we store just a relative displacement here.
	// Real-world code might do label resolution, etc.
	disp int32
}

func (op jmp) AppendAMD64(b []byte) []byte {
	// If disp fits in int8 => short jump
	if op.disp >= -128 && op.disp <= 127 {
		return append(b, 0xEB, byte(op.disp))
	}
	// else use near jump
	return append(b, 0xE9,
		byte(op.disp),
		byte(op.disp>>8),
		byte(op.disp>>16),
		byte(op.disp>>24))
}

// -- CALL --------------------------------------------------------------
// call rel32 => 0xE8, disp32
// (Near call, relative)
type call struct {
	disp int32
}

func (op call) AppendAMD64(b []byte) []byte {
	// call rel32 => 0xE8 disp32
	return append(b,
		0xE8,
		byte(op.disp),
		byte(op.disp>>8),
		byte(op.disp>>16),
		byte(op.disp>>24))
}

// -- MUL ---------------------------------------------------------------
// mul r/m8 => 0xF6 /4
// mul r/m16 => [66] 0xF7 /4
// mul r/m32 => 0xF7 /4
// mul r/m64 => REX.W + 0xF7 /4
type mul[A any] struct {
	r A
}

func (op mul[A]) AppendAMD64(b []byte) []byte {
	// This form does an unsigned multiply of RAX and r/mX -> RDX:RAX
	// For demonstration, just handle r8 or r64
	switch r := any(op.r).(type) {
	case Register[uint8]:
		// mul r8 => 0xF6 /4
		return append(b, 0xF6, 0xE0|byte(r))
	case Register[uint64]:
		// mul r64 => 0x48, 0xF7 /4
		rex := byte(0x48)
		modrm := byte(0xE0 | (r & 7)) // /4 => 100b
		if r >= 8 {
			rex |= 0x01
		}
		return append(b, rex, 0xF7, modrm)
	default:
		panic(fmt.Sprintf("MUL: unsupported register type %T", op.r))
	}
}

// -- DIV ---------------------------------------------------------------
// div r/m8 => 0xF6 /6
// div r/m16 => [66] 0xF7 /6
// div r/m32 => 0xF7 /6
// div r/m64 => REX.W + 0xF7 /6
type divInstr[A any] struct {
	r A
}

func (op divInstr[A]) AppendAMD64(b []byte) []byte {
	// This form does unsigned divide of RDX:RAX by r/mX, storing the remainder in RDX and quotient in RAX
	// Example coverage for r8 and r64
	switch r := any(op.r).(type) {
	case Register[uint8]:
		// div r8 => 0xF6 /6
		return append(b, 0xF6, 0xF0|byte(r)) // /6 => 110b in modrm.reg
	case Register[uint64]:
		// div r64 => 0x48, 0xF7 /6
		rex := byte(0x48)
		modrm := byte(0xF0 | (r & 7))
		if r >= 8 {
			rex |= 0x01
		}
		return append(b, rex, 0xF7, modrm)
	default:
		panic(fmt.Sprintf("DIV: unsupported register type %T", op.r))
	}
}

// -- AND ---------------------------------------------------------------
// and r/m8, r8 => 0x20 /r
// and r/m16, r16 => [66] 0x21 /r
// and r/m32, r32 => 0x21 /r
// and r/m64, r64 => REX.W + 0x21 /r
type and[A, B any] struct {
	args xyz.Pair[A, B]
}

func (op and[A, B]) AppendAMD64(b []byte) []byte {
	switch args := any(op.args).(type) {
	// Register-to-Register
	case xyz.Pair[Register[uint8], Register[uint8]]:
		dst, src := args.Split()
		return append(b, 0x20, 0xC0|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint16], Register[uint16]]:
		dst, src := args.Split()
		return append(b, 0x66, 0x21, 0xC0|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint32], Register[uint32]]:
		dst, src := args.Split()
		return append(b, 0x21, 0xC0|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint64], Register[uint64]]:
		dst, src := args.Split()
		rex := byte(0x48) // REX.W for 64-bit
		if dst >= 8 {
			rex |= 0x04 // REX.R for dst
		}
		if src >= 8 {
			rex |= 0x01 // REX.B for src
		}
		return append(b, rex, 0x21, 0xC0|byte(src&0x07)<<3|byte(dst&0x07))

	// Register-to-Immediate
	case xyz.Pair[Register[uint8], Imm8]:
		dst, src := args.Split()
		if dst == AL {
			return append(b, 0x24, byte(src))
		}
		return append(b, 0x80, 0xE0|byte(dst), byte(src))
	case xyz.Pair[Register[uint16], Imm16]:
		dst, src := args.Split()
		if dst == AX {
			return append(b, 0x66, 0x25, byte(src), byte(src>>8))
		}
		return append(b, 0x66, 0x81, 0xE0|byte(dst), byte(src), byte(src>>8))
	case xyz.Pair[Register[uint32], Imm32]:
		dst, src := args.Split()
		if dst == EAX {
			return append(b, 0x25, byte(src), byte(src>>8), byte(src>>16), byte(src>>24))
		}
		return append(b, 0x81, 0xE0|byte(dst), byte(src), byte(src>>8), byte(src>>16), byte(src>>24))
	case xyz.Pair[Register[uint64], Imm32]:
		dst, src := args.Split()
		rex := byte(0x48)
		if dst >= 8 {
			rex |= 0x04
		}
		return append(b, rex, 0x81, 0xE0|byte(dst&0x07), byte(src), byte(src>>8), byte(src>>16), byte(src>>24))

	default:
		panic(fmt.Sprintf("AND: unsupported operands: %T", op.args))
	}
}

// -- OR ----------------------------------------------------------------
// or r/m8, r8 => 0x08 /r
// or r/m16, r16 => [66] 0x09 /r
// or r/m32, r32 => 0x09 /r
// or r/m64, r64 => REX.W + 0x09 /r
type or[A, B any] struct {
	args xyz.Pair[A, B]
}

func (op or[A, B]) AppendAMD64(b []byte) []byte {
	switch args := any(op.args).(type) {
	// Register-to-Register
	case xyz.Pair[Register[uint8], Register[uint8]]:
		dst, src := args.Split()
		return append(b, 0x08, 0xC0|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint16], Register[uint16]]:
		dst, src := args.Split()
		return append(b, 0x66, 0x09, 0xC0|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint32], Register[uint32]]:
		dst, src := args.Split()
		return append(b, 0x09, 0xC0|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint64], Register[uint64]]:
		dst, src := args.Split()
		rex := byte(0x48) // REX.W for 64-bit
		if dst >= 8 {
			rex |= 0x04 // REX.R for dst
		}
		if src >= 8 {
			rex |= 0x01 // REX.B for src
		}
		return append(b, rex, 0x09, 0xC0|byte(src&0x07)<<3|byte(dst&0x07))

	// Register-to-Immediate
	case xyz.Pair[Register[uint8], Imm8]:
		dst, src := args.Split()
		if dst == AL {
			return append(b, 0x0C, byte(src))
		}
		return append(b, 0x80, 0xC8|byte(dst), byte(src))
	case xyz.Pair[Register[uint16], Imm16]:
		dst, src := args.Split()
		if dst == AX {
			return append(b, 0x66, 0x0D, byte(src), byte(src>>8))
		}
		return append(b, 0x66, 0x81, 0xC8|byte(dst), byte(src), byte(src>>8))
	case xyz.Pair[Register[uint32], Imm32]:
		dst, src := args.Split()
		if dst == EAX {
			return append(b, 0x0D, byte(src), byte(src>>8), byte(src>>16), byte(src>>24))
		}
		return append(b, 0x81, 0xC8|byte(dst), byte(src), byte(src>>8), byte(src>>16), byte(src>>24))
	case xyz.Pair[Register[uint64], Imm32]:
		dst, src := args.Split()
		rex := byte(0x48)
		if dst >= 8 {
			rex |= 0x04
		}
		return append(b, rex, 0x81, 0xC8|byte(dst&0x07), byte(src), byte(src>>8), byte(src>>16), byte(src>>24))

	default:
		panic(fmt.Sprintf("OR: unsupported operands: %T", op.args))
	}
}

// -- XOR ---------------------------------------------------------------
// xor r/m8, r8 => 0x30 /r
// xor r/m16, r16 => [66] 0x31 /r
// xor r/m32, r32 => 0x31 /r
// xor r/m64, r64 => REX.W + 0x31 /r
type xor[A, B any] struct {
	args xyz.Pair[A, B]
}

func (op xor[A, B]) AppendAMD64(b []byte) []byte {
	switch args := any(op.args).(type) {
	// Register-to-Register
	case xyz.Pair[Register[uint8], Register[uint8]]:
		dst, src := args.Split()
		return append(b, 0x30, 0xC0|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint16], Register[uint16]]:
		dst, src := args.Split()
		return append(b, 0x66, 0x31, 0xC0|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint32], Register[uint32]]:
		dst, src := args.Split()
		return append(b, 0x31, 0xC0|byte(src)<<3|byte(dst))
	case xyz.Pair[Register[uint64], Register[uint64]]:
		dst, src := args.Split()
		rex := byte(0x48) // REX.W for 64-bit
		if dst >= 8 {
			rex |= 0x04 // REX.R for dst
		}
		if src >= 8 {
			rex |= 0x01 // REX.B for src
		}
		return append(b, rex, 0x31, 0xC0|byte(src&0x07)<<3|byte(dst&0x07))

	// Register-to-Immediate
	case xyz.Pair[Register[uint8], Imm8]:
		dst, src := args.Split()
		if dst == AL {
			return append(b, 0x34, byte(src))
		}
		return append(b, 0x80, 0xF0|byte(dst), byte(src))
	case xyz.Pair[Register[uint16], Imm16]:
		dst, src := args.Split()
		if dst == AX {
			return append(b, 0x66, 0x35, byte(src), byte(src>>8))
		}
		return append(b, 0x66, 0x81, 0xF0|byte(dst), byte(src), byte(src>>8))
	case xyz.Pair[Register[uint32], Imm32]:
		dst, src := args.Split()
		if dst == EAX {
			return append(b, 0x35, byte(src), byte(src>>8), byte(src>>16), byte(src>>24))
		}
		return append(b, 0x81, 0xF0|byte(dst), byte(src), byte(src>>8), byte(src>>16), byte(src>>24))
	case xyz.Pair[Register[uint64], Imm32]:
		dst, src := args.Split()
		rex := byte(0x48)
		if dst >= 8 {
			rex |= 0x04
		}
		return append(b, rex, 0x81, 0xF0|byte(dst&0x07), byte(src), byte(src>>8), byte(src>>16), byte(src>>24))

	default:
		panic(fmt.Sprintf("XOR: unsupported operands: %T", op.args))
	}
}

// Helper functions for bit manipulation instructions
func And[A, B any](dst A, src B) and[A, B] {
	return and[A, B]{args: xyz.Pair[A, B]{dst, src}}
}

func Or[A, B any](dst A, src B) or[A, B] {
	return or[A, B]{args: xyz.Pair[A, B]{dst, src}}
}

func Xor[A, B any](dst A, src B) xor[A, B] {
	return xor[A, B]{args: xyz.Pair[A, B]{dst, src}}
}
