package amd64

import (
	"runtime.link/cpu"
)

const (
	RAX cpu.GPR = iota
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
	cpu.Architecture `cpu:"amd64"`

	Math struct {
		Add func(a, b cpu.GPR) `cpu:"01001[b>7]0[a>7] 0x01 11bbbaaa"`
		Sub func(a, b cpu.GPR) `cpu:"01001[b>7]1[a>7] 0x29 11bbbaaa"`
		Mul func(a cpu.GPR)    `cpu:"0100101[a>7]     0xF7 1110000aaa"` // RDX:RAX = RAX*by
		Div func(a cpu.GPR)    `cpu:"0100101[a>7]     0xF7 1111000aaa"` // RAX = RDX:RAX/by, RDX = RDX:RAX%by
	}
	Return func() `cpu:"0xC3"`
}
