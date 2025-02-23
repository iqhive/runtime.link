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
