package cpu

type registers struct {
	RegistersLong
	r6, r7, r8                                          Register
	x3, x4, x5, x6, x7, x8, x9, x10, x11, x12, x13, x14 FloatingPointRegister
}

func (r *registers) R6() *Register               { return &r.r6 }
func (r *registers) R7() *Register               { return &r.r7 }
func (r *registers) R8() *Register               { return &r.r8 }
func (r *registers) X3() *FloatingPointRegister  { return &r.x3 }
func (r *registers) X4() *FloatingPointRegister  { return &r.x4 }
func (r *registers) X5() *FloatingPointRegister  { return &r.x5 }
func (r *registers) X6() *FloatingPointRegister  { return &r.x6 }
func (r *registers) X7() *FloatingPointRegister  { return &r.x7 }
func (r *registers) X8() *FloatingPointRegister  { return &r.x8 }
func (r *registers) X9() *FloatingPointRegister  { return &r.x9 }
func (r *registers) X10() *FloatingPointRegister { return &r.x10 }
func (r *registers) X11() *FloatingPointRegister { return &r.x11 }
func (r *registers) X12() *FloatingPointRegister { return &r.x12 }
func (r *registers) X13() *FloatingPointRegister { return &r.x13 }
func (r *registers) X14() *FloatingPointRegister { return &r.x14 }
