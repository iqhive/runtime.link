// Package arm64 provides an instruction set specification for the AMD64 architecture.
package arm64

import (
	"runtime.link/cpu"
)

const (
	X0 cpu.GPR = iota
	X1
	X2
	X3
	X4
	X5
	X6
	X7
	X8
	X9
	X10
	X11
	X12
	X13
	X14
	X15
	X16
	X17
	X18
	X19
	X20
	X21
	X22
	X23
	X24
	X25
	X26
	X27
	X28
	X29
	X30
	X31
	SP
)

// InstructionSet specification.
// https://developer.arm.com/documentation/ddi0602/2023-06/Base-Instructions
type InstructionSet struct {
	cpu.Architecture `cpu:"arm64,reverse"` // reversed byte order to match ARM documentation table bits.

	Math struct {
		Add func(a, b, c cpu.GPR) `cpu:"10001011 000ccccc 000000bb bbbaaaaa"`
	}
	Return func() `cpu:"11010110 01011111 00000011 11000000"`
}
