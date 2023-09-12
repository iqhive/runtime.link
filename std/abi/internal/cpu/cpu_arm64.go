package cpu

type frame struct {
	r0, r1, r2, r3, r4, r5, r6, r7 Register
	x0, x1, x2, x3, x4, x5, x6, x7 FloatingPointRegister
}

func (frame) Registers() int { return 16 }

func (frame) FloatingPointRegisters() int { return 16 }
