package amd64

import (
	"fmt"

	"runtime.link/xyz"
)

type literal string

const (
	AAA literal = "\x37" // ASCII Adjust After Addition
	AAS literal = "\x3F" // ASCII Adjust AL After Subtraction
	RET literal = "\xC3" // Return
)

func (l literal) AppendBinary(b []byte) ([]byte, error) { return append(b, l...), nil }

type (
	AAD uint8 // ASCII Adjust AX Before Division
	AAM uint8 // ASCII Adjust AX After Multiply
)

func (op AAD) AppendBinary(b []byte) ([]byte, error) { return append(b, 0xD5, byte(op)), nil }
func (op AAM) AppendBinary(b []byte) ([]byte, error) { return append(b, 0xD4, byte(op)), nil }

type canBeAddedTo[A any] interface {
	canAddTo(A)
}

type ADC[
	A AnyRegister | AnyPointer,
	B canBeAddedTo[A],
] struct {
	Args xyz.Pair[A, B]
}

func (op ADC[A, B]) AppendBinary(b []byte) ([]byte, error) {
	switch args := any(op.Args).(type) {
	// Register-to-Register
	case xyz.Pair[Register[uint8], Register[uint8]]:
		dst, src := args.Split()
		return append(b, 0x10, 0xC0|byte(dst)<<3|byte(src)), nil
	case xyz.Pair[Register[uint16], Register[uint16]]:
		dst, src := args.Split()
		return append(b, 0x66, 0x11, 0xC0|byte(dst)<<3|byte(src)), nil
	case xyz.Pair[Register[uint32], Register[uint32]]:
		dst, src := args.Split()
		return append(b, 0x11, 0xC0|byte(dst)<<3|byte(src)), nil
	case xyz.Pair[Register[uint64], Register[uint64]]:
		dst, src := args.Split()
		rex := byte(0x48) // REX.W for 64-bit
		if dst >= 8 {
			rex |= 0x04 // REX.R for dst
		}
		if src >= 8 {
			rex |= 0x01 // REX.B for src
		}
		return append(b, rex, 0x11, 0xC0|byte(dst&0x07)<<3|byte(src&0x07)), nil
	// Register-to-Immediate
	case xyz.Pair[Register[uint8], Imm8]:
		dst, src := args.Split()
		if dst == AL {
			return append(b, 0x14, byte(src)), nil
		}
		return append(b, 0x80, 0xD0|byte(dst), byte(src)), nil
	case xyz.Pair[Register[uint16], Imm8]:
		dst, src := args.Split()
		return append(b, 0x66, 0x83, 0xD0|byte(dst), byte(src)), nil // Sign-extended imm8
	case xyz.Pair[Register[uint16], Imm16]:
		dst, src := args.Split()
		if dst == AX {
			return append(b, 0x66, 0x15, byte(src), byte(src>>8)), nil
		}
		return append(b, 0x66, 0x81, 0xD0|byte(dst), byte(src), byte(src>>8)), nil
	case xyz.Pair[Register[uint32], Imm8]:
		dst, src := args.Split()
		return append(b, 0x83, 0xD0|byte(dst), byte(src)), nil
	case xyz.Pair[Register[uint32], Imm16]:
		dst, src := args.Split()
		return append(b, 0x81, 0xD0|byte(dst), byte(src), byte(src>>8)), nil
	case xyz.Pair[Register[uint32], Imm32]:
		dst, src := args.Split()
		if dst == EAX {
			return append(b, 0x05, byte(src), byte(src>>8), byte(src>>16), byte(src>>24)), nil
		}
		return append(b, 0x81, 0xD0|byte(dst), byte(src), byte(src>>8), byte(src>>16), byte(src>>24)), nil
	// Register-to-Memory
	case xyz.Pair[Register[uint8], Pointer[uint8]]:
		dst, src := args.Split()
		return append(b, 0x12, 0x00|byte(dst)<<3|byte(src)), nil
	case xyz.Pair[Register[uint16], Pointer[uint16]]:
		dst, src := args.Split()
		return append(b, 0x66, 0x13, 0x00|byte(dst)<<3|byte(src)), nil
	case xyz.Pair[Register[uint32], Pointer[uint32]]:
		dst, src := args.Split()
		return append(b, 0x13, 0x00|byte(dst)<<3|byte(src)), nil
	case xyz.Pair[Register[uint64], Pointer[uint64]]:
		dst, src := args.Split()
		rex := byte(0x48) // REX.W
		if dst >= 8 {
			rex |= 0x04 // REX.R
		}
		if src >= 8 {
			rex |= 0x01 // REX.B
		}
		return append(b, rex, 0x13, 0x00|byte(dst&0x07)<<3|byte(src&0x07)), nil
	// Memory-to-Register
	case xyz.Pair[Pointer[uint8], Register[uint8]]:
		dst, src := args.Split()
		return append(b, 0x10, 0x00|byte(src)<<3|byte(dst)), nil
	case xyz.Pair[Pointer[uint16], Register[uint16]]:
		dst, src := args.Split()
		return append(b, 0x66, 0x11, 0x00|byte(src)<<3|byte(dst)), nil
	case xyz.Pair[Pointer[uint32], Register[uint32]]:
		dst, src := args.Split()
		return append(b, 0x11, 0x00|byte(src)<<3|byte(dst)), nil
	case xyz.Pair[Pointer[uint64], Register[uint64]]:
		dst, src := args.Split()
		rex := byte(0x48) // REX.W
		if src >= 8 {
			rex |= 0x04 // REX.R
		}
		if dst >= 8 {
			rex |= 0x01 // REX.B
		}
		return append(b, rex, 0x11, 0x00|byte(src&0x07)<<3|byte(dst&0x07)), nil
	// Memory-to-Immediate
	case xyz.Pair[Pointer[uint8], Imm8]:
		dst, src := args.Split()
		return append(b, 0x80, 0x50|byte(dst), byte(src)), nil
	case xyz.Pair[Pointer[uint16], uint16]:
		dst, src := args.Split()
		return append(b, 0x81, 0x50|byte(dst), uint8(src), uint8(src>>8)), nil
	case xyz.Pair[Pointer[uint32], uint32]:
		dst, src := args.Split()
		return append(b, 0x81, 0x50|byte(dst), uint8(src), uint8(src>>8), uint8(src>>16), uint8(src>>24)), nil
	default:
		return nil, fmt.Errorf("invalid arguments: %T", op.Args)
	}
}
