package cpu

// Registers represent the full set of calling registers available on the
// current architecture for handling the slowest but most general calls.
type RegistersArch = registers

// RegistersFast represent the smallest set of useful calling registers
// available on the current architecture for the fastest calls (R0, R1, and X0).
type RegistersFast struct {
	RegistersZero

	r0, r1 Register
	x0     FloatingPointRegister
}

func (r *RegistersFast) R0() *Register              { return &r.r0 }
func (r *RegistersFast) R1() *Register              { return &r.r1 }
func (r *RegistersFast) X0() *FloatingPointRegister { return &r.x0 }

// RegistersSome represent a useful set of calling registers available on the
// current architecture for faster calls (R0, R1, R2, R3, X0 and X1).
type RegistersSome struct {
	RegistersFast
	r2, r3 Register
	x1     FloatingPointRegister
}

func (r *RegistersSome) R2() *Register              { return &r.r2 }
func (r *RegistersSome) R3() *Register              { return &r.r3 }
func (r *RegistersSome) X1() *FloatingPointRegister { return &r.x1 }

// RegistersLong represent a suitable set of calling registers available on the
// current architecture for most calls (R0, R1, R2, R3, R4, R5, X0, X1 and X2).
type RegistersLong struct {
	RegistersSome
	r4, r5 Register
	x2     FloatingPointRegister
}

func (r *RegistersLong) R4() *Register              { return &r.r4 }
func (r *RegistersLong) R5() *Register              { return &r.r5 }
func (r *RegistersLong) X2() *FloatingPointRegister { return &r.x2 }

// RegistersMany represent even more registers that are available on the
// current architecture for calls that should still be slightly faster than
// the full register set (R0, R1, R2, R3, R4, R5, R6, R7, X0, X1, X2 and X3).
type RegistersMany struct {
	RegistersLong
	r6, r7 Register
	x3     FloatingPointRegister
}

func (r *RegistersMany) R6() *Register              { return &r.r6 }
func (r *RegistersMany) R7() *Register              { return &r.r7 }
func (r *RegistersMany) X3() *FloatingPointRegister { return &r.x3 }

// RegistersFull represent the full set of calling registers, even if they
// are not available on the current architecture (undefined behaviour).
type RegistersFull struct {
	RegistersMany
	r8, r9, r10, r11, r12, r13, r14, r15                 Register
	x4, x5, x6, x7, x8, x9, x10, x11, x12, x13, x14, x15 FloatingPointRegister
}

func (r *RegistersFull) R8() *Register               { return &r.r8 }
func (r *RegistersFull) R9() *Register               { return &r.r9 }
func (r *RegistersFull) R10() *Register              { return &r.r10 }
func (r *RegistersFull) R11() *Register              { return &r.r11 }
func (r *RegistersFull) R12() *Register              { return &r.r12 }
func (r *RegistersFull) R13() *Register              { return &r.r13 }
func (r *RegistersFull) R14() *Register              { return &r.r14 }
func (r *RegistersFull) R15() *Register              { return &r.r15 }
func (r *RegistersFull) X4() *FloatingPointRegister  { return &r.x4 }
func (r *RegistersFull) X5() *FloatingPointRegister  { return &r.x5 }
func (r *RegistersFull) X6() *FloatingPointRegister  { return &r.x6 }
func (r *RegistersFull) X7() *FloatingPointRegister  { return &r.x7 }
func (r *RegistersFull) X8() *FloatingPointRegister  { return &r.x8 }
func (r *RegistersFull) X9() *FloatingPointRegister  { return &r.x9 }
func (r *RegistersFull) X10() *FloatingPointRegister { return &r.x10 }
func (r *RegistersFull) X11() *FloatingPointRegister { return &r.x11 }
func (r *RegistersFull) X12() *FloatingPointRegister { return &r.x12 }
func (r *RegistersFull) X13() *FloatingPointRegister { return &r.x13 }
func (r *RegistersFull) X14() *FloatingPointRegister { return &r.x14 }
func (r *RegistersFull) X15() *FloatingPointRegister { return &r.x15 }

// RegistersZero represents access to no calling registers. Accessing any
// register will result in a panic.
type RegistersZero struct{}

func (r *RegistersZero) R0() *Register               { panic("R0 not available") }
func (r *RegistersZero) R1() *Register               { panic("R1 not available") }
func (r *RegistersZero) R2() *Register               { panic("R2 not available") }
func (r *RegistersZero) R3() *Register               { panic("R3 not available") }
func (r *RegistersZero) R4() *Register               { panic("R4 not available") }
func (r *RegistersZero) R5() *Register               { panic("R5 not available") }
func (r *RegistersZero) R6() *Register               { panic("R6 not available") }
func (r *RegistersZero) R7() *Register               { panic("R7 not available") }
func (r *RegistersZero) R8() *Register               { panic("R8 not available") }
func (r *RegistersZero) R9() *Register               { panic("R9 not available") }
func (r *RegistersZero) R10() *Register              { panic("R10 not available") }
func (r *RegistersZero) R11() *Register              { panic("R11 not available") }
func (r *RegistersZero) R12() *Register              { panic("R12 not available") }
func (r *RegistersZero) R13() *Register              { panic("R13 not available") }
func (r *RegistersZero) R14() *Register              { panic("R14 not available") }
func (r *RegistersZero) R15() *Register              { panic("R15 not available") }
func (r *RegistersZero) X0() *FloatingPointRegister  { panic("X0 not available") }
func (r *RegistersZero) X1() *FloatingPointRegister  { panic("X1 not available") }
func (r *RegistersZero) X2() *FloatingPointRegister  { panic("X2 not available") }
func (r *RegistersZero) X3() *FloatingPointRegister  { panic("X3 not available") }
func (r *RegistersZero) X4() *FloatingPointRegister  { panic("X4 not available") }
func (r *RegistersZero) X5() *FloatingPointRegister  { panic("X5 not available") }
func (r *RegistersZero) X6() *FloatingPointRegister  { panic("X6 not available") }
func (r *RegistersZero) X7() *FloatingPointRegister  { panic("X7 not available") }
func (r *RegistersZero) X8() *FloatingPointRegister  { panic("X8 not available") }
func (r *RegistersZero) X9() *FloatingPointRegister  { panic("X9 not available") }
func (r *RegistersZero) X10() *FloatingPointRegister { panic("X10 not available") }
func (r *RegistersZero) X11() *FloatingPointRegister { panic("X11 not available") }
func (r *RegistersZero) X12() *FloatingPointRegister { panic("X12 not available") }
func (r *RegistersZero) X13() *FloatingPointRegister { panic("X13 not available") }
func (r *RegistersZero) X14() *FloatingPointRegister { panic("X14 not available") }
func (r *RegistersZero) X15() *FloatingPointRegister { panic("X15 not available") }

type Registers interface {
	R0() *Register
	R1() *Register
	R2() *Register
	R3() *Register
	R4() *Register
	R5() *Register
	R6() *Register
	R7() *Register
	R8() *Register
	R9() *Register
	R10() *Register
	R11() *Register
	R12() *Register
	R13() *Register
	R14() *Register
	R15() *Register
	X0() *FloatingPointRegister
	X1() *FloatingPointRegister
	X2() *FloatingPointRegister
	X3() *FloatingPointRegister
	X4() *FloatingPointRegister
	X5() *FloatingPointRegister
	X6() *FloatingPointRegister
	X7() *FloatingPointRegister
	X8() *FloatingPointRegister
	X9() *FloatingPointRegister
	X10() *FloatingPointRegister
	X11() *FloatingPointRegister
	X12() *FloatingPointRegister
	X13() *FloatingPointRegister
	X14() *FloatingPointRegister
	X15() *FloatingPointRegister
}
