package amd64

import "runtime.link/jit"

type GPR byte

const (
	RAX GPR = iota
	RCX
	RDX
	RBX
	RSP
	RBP
	RSI
	RDI

	R8
	R9
	R10
	R11
	R12
	R13
	R14
	R15
)

// InstructionSet specification.
type InstructionSet struct {
	jit.Architecture `jit:"amd64"`

	Math struct {
		Add func(a, b GPR) `asm:"01001[b>7]0[a>7] 0x01 11[b000][a000]"`
		Sub func(a, b GPR) `asm:"01001[b>7]1[a>7] 0x29 11[b000][a000]"`
		Mul func(a GPR)    `asm:"0100101[a>7]     0xF7 1110000[a000]"` // RDX:RAX = RAX*by
		Div func(a GPR)    `asm:"0100101[a>7]     0xF7 1111000[a000]"` // RAX = RDX:RAX/by, RDX = RDX:RAX%by
	}
	Return func() `asm:"0xC3"`
}
