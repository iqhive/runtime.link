//go:build !amd64 && !arm64

package cpu

// The registers that Go's internal ABI assigns to values are only known here
// for amd64 and arm64. Anywhere else no value is given one, which is how the
// package already treats a value that is out of reach of a Program, so that it
// builds for those architectures without claiming to work on them.
const (
	generalRegisters  = 0
	floatingRegisters = 0
)

type registers = RegistersFull
