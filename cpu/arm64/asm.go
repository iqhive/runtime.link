package arm64

type Instruction uint32

type features uint32

const (
	cssc features = 1 << iota
)

// use is a from and upto register
// along with a read/write pair of
// bits followed by a 4-bit size.
type use uint16

type Assembly struct {
	code uint32
	feat features
	uses [4]use
}

func size[T anyRegister](reg T) Instruction {
	switch any(reg).(type) {
	case Vector[[16]uint8]:
		return 0
	case Vector[[8]uint16]:
		return 1
	case Vector[[4]uint32]:
		return 2
	}
	return 3
}

func rd[T anyRegister](reg T) Instruction { return Instruction(reg&0b11111) << 0 }
func rn[T anyRegister](reg T) Instruction { return Instruction(reg&0b11111) << 5 }
func rm[T anyRegister](reg T) Instruction { return Instruction(reg&0b11111) << 16 }

func imm2[T ~uint8](val T) Instruction   { return Instruction(val & 0b11) }
func imm3[T ~uint8](val T) Instruction   { return Instruction(val & 0b111) }
func imm4[T ~uint8](val T) Instruction   { return Instruction(val & 0b1111) }
func imm5[T ~uint8](val T) Instruction   { return Instruction(val & 0b11111) }
func imm6[T ~uint8](val T) Instruction   { return Instruction(val & 0b111111) }
func imm7[T ~uint8](val T) Instruction   { return Instruction(val & 0b1111111) }
func imm8[T ~uint8](val T) Instruction   { return Instruction(val & 0b11111111) }
func imm9[T ~uint16](val T) Instruction  { return Instruction(val & 0b111111111) }
func imm10[T ~uint16](val T) Instruction { return Instruction(val & 0b1111111111) }
func imm11[T ~uint16](val T) Instruction { return Instruction(val & 0b11111111111) }
func imm12[T ~uint16](val T) Instruction { return Instruction(val & 0b111111111111) }
func imm13[T ~uint16](val T) Instruction { return Instruction(val & 0b1111111111111) }
func imm14[T ~uint16](val T) Instruction { return Instruction(val & 0b11111111111111) }
func imm16[T ~uint16](val T) Instruction { return Instruction(val & 0b1111111111111111) }
