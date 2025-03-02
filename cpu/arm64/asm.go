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
	case V[[16]uint8], V[[16]int8]:
		return 0
	case V[[8]uint16], V[[8]int16]:
		return 1
	case V[[4]uint32], V[[4]int32], V[[4]float32]:
		return 2
	}
	return 3
}

func sf[T anyRegister](reg T) Instruction {
	switch any(reg).(type) {
	case X[float64], X[uint64], X[int64]:
		return 1 << 31
	}
	return 0
}
func rd[T anyRegister](reg T) Instruction { return Instruction(reg&0b11111) << 0 }
func rn[T anyRegister](reg T) Instruction { return Instruction(reg&0b11111) << 5 }
func rm[T anyRegister](reg T) Instruction { return Instruction(reg&0b11111) << 16 }

func imm2[T ~uint8 | ~int8](val T) Instruction {
	if val < 0 {
		return (Instruction(val&0b11) | 1<<2)
	}
	return Instruction(val & 0b11)
}
func imm3[T ~uint8 | ~int8](val T) Instruction {
	if val < 0 {
		return (Instruction(val&0b111) | 1<<3)
	}
	return Instruction(val & 0b111)
}
func imm4[T ~uint8 | ~int8](val T) Instruction {
	if val < 0 {
		return (Instruction(val&0b1111) | 1<<4)
	}
	return Instruction(val & 0b1111)
}
func imm6[T ~uint8 | ~int8](val T) Instruction {
	if val < 0 {
		return (Instruction(val&0b111111) | 1<<6)
	}
	return Instruction(val & 0b111111)
}
func imm12[T ~uint16 | ~int16](val T) Instruction {
	if val < 0 {
		return (Instruction(val&0b111111111111) | 1<<12)
	}
	return Instruction(val & 0b111111111111)
}

func immhi[T ~uint32 | ~int32](val T) Instruction {
	if val < 0 {
		return (Instruction(val&0b011111111111111111100) | 1<<20) << 3
	}
	return Instruction(val&0b111111111111111111100) << 3
}
func immlo[T ~uint32 | ~int32](val T) Instruction {
	return Instruction(val&0b11) << 29
}
