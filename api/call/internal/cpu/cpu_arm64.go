package cpu

// The number of registers that Go's internal ABI assigns to values.
const (
	generalRegisters  = 16
	floatingRegisters = 16
)

type registers = RegistersFull
