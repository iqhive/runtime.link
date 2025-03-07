package arm64

import (
	"encoding/binary"
	"io"
)

type assembler struct {
	w io.Writer
}

func newAssembler(w io.Writer) assembler {
	return assembler{w: w}
}

func (asm assembler) API() API {
	return API{
		ABS:  func(dst, src Register[int64]) error { return asm.write(0b101101011000000001<<13 | rd(dst) | rn(src)) },
		ADC:  func(dst, a, b X) error { return asm.write(0b1001101<<25 | rd(dst) | rn(a) | rm(b)) },
		ADCS: func(dst, a, b X) error { return asm.write(0b1011101<<25 | rd(dst) | rn(a) | rm(b)) },
		ADD: NewExtendedImmediateShifted(
			func(dst, src1, src2 X, extension RegisterExtension, amount Uint3) error {
				return asm.write(0b10001011001<<21 | rd(dst) | rn(src1) | rm(src2) | imm3(amount)<<10 | imm3(extension)<<13)
			},
			func(dst, src X, val Imm12) error {
				return asm.write(0b10010001<<24 | rd(dst) | rn(src) | imm12(val)<<10)
			},
			func(dst, a, b X, shift Shift, amount Uint6) error {
				return asm.write(0b10001011<<24 | rd(dst) | rn(a) | imm6(amount)<<10 | imm2(shift)<<22 | rm(b))
			},
		),
		ADDG: func(dst, src Register[TaggedPointer], offset Int6, tag_offset Int4) error {
			return asm.write(0b100100011<<22 | rd(dst) | rn(src) | imm6(offset)<<16 | imm4(tag_offset)<<10)
		},
		ADDPT: func(dst, ptr Register[CheckedPointer], offset Register[int64], shift_amount Uint3) error {
			return asm.write(0b1001101<<25 | rd(dst) | rn(ptr) | imm3(shift_amount)<<10 | rm(offset))
		},
		ADDS: NewExtendedImmediateShifted(
			func(dst, src1, src2 Register[[8]byte], extension RegisterExtension, amount Uint3) error {
				return asm.write(0b10101011001<<21 | rd(dst) | rn(src1) | rm(src2) | imm3(amount)<<10 | imm3(extension)<<13)
			},
			func(dst, src Register[[8]byte], val Uint12) error {
				return asm.write(0b10110001<<24 | rd(dst) | rn(src) | imm12(val)<<10)
			},
			func(dst, a, b Register[[8]byte], shift Shift, amount Uint6) error {
				return asm.write(0b10101011<<24 | rd(dst) | rn(a) | imm6(amount)<<10 | imm2(shift)<<22 | rm(b))
			},
		),
		ADR: func(dst Register[ProgramCounter], offset Int21) error {
			return asm.write(0b0001<<28 | rd(dst) | immlo(offset) | immhi(offset))
		},
		ADRP: func(dst Register[ProgramCounter], offset Int21) error {
			return asm.write(0b1001<<28 | rd(dst) | immlo(offset) | immhi(offset))
		},
		AND: NewImmediateShifted(
			func(dst, src X, val BitPattern) error {
				n, immr, imms, err := val.encode()
				if err != nil {
					return err
				}
				return asm.write(0b1001001<<25 | rd(dst) | rn(src) | n<<22 | immr<<16 | imms<<10)
			},
			func(dst, a, b X, shift Shift, amount Uint6) error {
				return asm.write(0b1000101<<25 | rd(dst) | rn(a) | rm(b) | imm6(amount)<<10 | uint32(shift)<<22)
			},
		),
		ANDS: NewImmediateShifted(
			func(dst, src X, val BitPattern) error {
				n, immr, imms, err := val.encode()
				if err != nil {
					return err
				}
				return asm.write(0b1111001<<25 | rd(dst) | rn(src) | n<<22 | immr<<16 | imms<<10)
			},
			func(dst, a, b X, shift Shift, amount Uint6) error {
				return asm.write(0b1110101<<25 | rd(dst) | rn(a) | rm(b) | imm6(amount)<<10 | uint32(shift)<<22)
			},
		),
		APAS: func(src Register[uintptr]) error { return asm.write(0b11010101000011100111<<12 | rd(src)) },
		ASR: NewImmediateRegister(
			func(dst Register[int64], src Register[int64], amount Uint6) error {
				return asm.write(0b1001001101000000111111<<10 | rd(dst) | rn(src) | imm6(amount)<<16)
			},
			func(dst, a Register[int64], b Register[uint64]) error {
				return asm.write(0b100110101100000000101<<11 | rd(dst) | rn(a) | rm(b))
			},
		),
		AT: func(ptr Register[uintptr], stage Stage, exception_level Uint2, check AddressChecks) error {
			var op1, CRm, op2 uint32
			switch {
			case stage == Stage1 && exception_level == 1 && check == AddressCheckRead:
				op1, CRm, op2 = 0b000, 0b1000, 0b000
			case stage == Stage1 && exception_level == 1 && check == AddressCheckWrite:
				op1, CRm, op2 = 0b000, 0b1000, 0b001
			case stage == Stage1 && exception_level == 0 && check == AddressCheckRead:
				op1, CRm, op2 = 0b000, 0b1000, 0b010
			case stage == Stage1 && exception_level == 0 && check == AddressCheckWrite:
				op1, CRm, op2 = 0b000, 0b1000, 0b011
			case stage == Stage1 && exception_level == 1 && check&AddressCheckAuthentication != 0 && check&AddressCheckRead != 0:
				op1, CRm, op2 = 0b000, 0b1001, 0b000
			case stage == Stage1 && exception_level == 1 && check&AddressCheckAuthentication != 0 && check&AddressCheckWrite != 0:
				op1, CRm, op2 = 0b000, 0b1001, 0b001
			case stage == Stage1 && exception_level == 1 && check == AddressCheckAlignment:
				op1, CRm, op2 = 0b000, 0b1001, 0b010
			case stage == Stage1 && exception_level == 2 && check == AddressCheckRead:
				op1, CRm, op2 = 0b100, 0b1000, 0b000
			case stage == Stage1 && exception_level == 2 && check == AddressCheckWrite:
				op1, CRm, op2 = 0b100, 0b1000, 0b001
			case stage == Stage1|Stage2 && exception_level == 1 && check == AddressCheckRead:
				op1, CRm, op2 = 0b100, 0b1000, 0b100
			case stage == Stage1|Stage2 && exception_level == 1 && check == AddressCheckWrite:
				op1, CRm, op2 = 0b100, 0b1000, 0b101
			case stage == Stage1|Stage2 && exception_level == 0 && check == AddressCheckRead:
				op1, CRm, op2 = 0b100, 0b1000, 0b110
			case stage == Stage1|Stage2 && exception_level == 0 && check == AddressCheckWrite:
				op1, CRm, op2 = 0b100, 0b1000, 0b111
			case stage == Stage1 && exception_level == 2 && check == AddressCheckAlignment:
				op1, CRm, op2 = 0b100, 0b1001, 0b010
			case stage == Stage1 && exception_level == 3 && check == AddressCheckRead:
				op1, CRm, op2 = 0b110, 0b1000, 0b000
			case stage == Stage1 && exception_level == 3 && check == AddressCheckWrite:
				op1, CRm, op2 = 0b110, 0b1000, 0b001
			case stage == Stage1 && exception_level == 3 && check == AddressCheckAlignment:
				op1, CRm, op2 = 0b110, 0b1001, 0b010
			}
			return asm.write(0b110101010000100001111<<11 | rd(ptr) | op1<<16 | op2<<5 | CRm<<8)
		},
		AUTDA: func(dst Register[uintptr], key X) error {
			return asm.write(0b110110101100000100011<<11 | rd(dst) | rn(key))
		},
		AUTDZA: func(dst Register[uintptr]) error { return asm.write(0b110110101100000100111<<11 | rd(dst)) },
		AUTDB: func(dst Register[uintptr], key X) error {
			return asm.write(0b1101101011000001000111<<10 | rd(dst) | rn(key))
		},
		AUTDZB: func(dst Register[uintptr]) error {
			return asm.write(0b1101101011000001001111<<10 | rd(dst))
		},
		AUTIA: func(dst Register[ProgramCounter], key X) error {
			return asm.write(0b11011010110000010001<<12 | rd(dst) | rn(key))
		},
		AUTIZA:      func(dst Register[ProgramCounter]) error { return asm.write(0b11011010110000010011<<12 | rd(dst)) },
		AUTIAZ:      func() error { return asm.write(0b11010101000000110010001110011111) },
		AUTIASP:     func() error { return asm.write(0b11010101000000110010001110111111) },
		AUTIA1716:   func() error { return asm.write(0b11010101000000110010001110011111) },
		AUTIA171615: func() error { return asm.write(0b11011010110000011011101111111110) },
		AUTIASPPC: func(pc_offset int16) error {
			return asm.write(0b111100111000000000000000011111 | imm16(pc_offset)<<5)
		},
		AUTIASPPCR: func(key X) error { return asm.write(0b11011010110000011001000000011110 | rn(key)) },
		AUTIB: func(dst Register[ProgramCounter], key X) error {
			return asm.write(0b1101101011000001000101<<10 | rd(dst) | rn(key))
		},
		AUTIZB:      func(dst Register[ProgramCounter]) error { return asm.write(0b110110101100000100110111111<<5 | rd(dst)) },
		AUTIBZ:      func() error { return asm.write(0b11010101000000110010000111011111) },
		AUTIB1716:   func() error { return asm.write(0b11010101000000110010001111111111) },
		AUTIBSP:     func() error { return asm.write(0b11010101000000110010001111011111) },
		AUTIB171615: func() error { return asm.write(0b11011010110000011011111111111110) },
		AUTIBSPPC: func(pc_offset int16) error {
			return asm.write(0b111100111010000000000000011111 | imm16(pc_offset)<<5)
		},
		AUTIBSPPCR: func(key X) error { return asm.write(0b11011010110000011001010000011110 | rn(key)) },
		AXFLAG:     func() error { return asm.write(0b11010101000000000100000001011111) },

		CMPs: func(src1, src2 X, shift Shift, amount Uint6) error {
			return asm.write(0b11101011<<24 | rn(src1) | rm(src2) | imm6(amount)<<10 | uint32(shift)<<22)
		},
		CSET: func(dst Register[bool], condition Condition) error {
			return asm.write(0b100110101001111100000111111<<5 | rd(dst) | imm4(condition)<<12)
		},
		RET: func(lnk Register[ProgramCounter]) error { return asm.write(0b1101011001011111<<16 | rn(lnk)) },
	}
}

func (asm assembler) write(instruction uint32) error {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], instruction)
	_, err := asm.w.Write(buf[:])
	return err
}

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

func rd[T uint64 | int64 | [8]byte | uintptr | CheckedPointer | TaggedPointer | bool | ProgramCounter](reg Register[T]) uint32 {
	return uint32(reg&0b11111) << 0
}
func rn[T uint64 | int64 | [8]byte | uintptr | CheckedPointer | TaggedPointer | bool | ProgramCounter](reg Register[T]) uint32 {
	return uint32(reg&0b11111) << 5
}
func rm[T uint64 | int64 | [8]byte | uintptr | CheckedPointer | TaggedPointer | bool | ProgramCounter](reg Register[T]) uint32 {
	return uint32(reg&0b11111) << 16
}

func imm1[T ~uint16 | ~int16](val T) uint32 {
	if val < 0 {
		return (uint32(val&0b1) | 1<<12)
	}
	return uint32(val & 0b1)
}

func imm2[T ~uint8 | ~int8 | ~uint32 | ~int32](val T) uint32 {
	if val < 0 {
		return (uint32(val&0b11) | 1<<2)
	}
	return uint32(val & 0b11)
}
func imm3[T ~uint8 | ~int8](val T) uint32 {
	if val < 0 {
		return (uint32(val&0b111) | 1<<3)
	}
	return uint32(val & 0b111)
}
func imm4[T ~uint8 | ~int8](val T) uint32 {
	if val < 0 {
		return (uint32(val&0b1111) | 1<<4)
	}
	return uint32(val & 0b1111)
}
func imm6[T ~uint8 | ~int8 | ~uint16 | ~int16](val T) uint32 {
	if val < 0 {
		return (uint32(val&0b111111) | 1<<6)
	}
	return uint32(val & 0b111111)
}
func imm12[T ~uint16 | ~int16](val T) uint32 {
	if val < 0 {
		return (uint32(val&0b111111111111) | 1<<12)
	}
	return uint32(val & 0b111111111111)
}
func imm13[T ~uint16 | ~int16](val T) uint32 {
	if val < 0 {
		return (uint32(val&0b1111111111111) | 1<<13)
	}
	return uint32(val & 0b0111111111111)
}

func immhi[T ~uint32 | ~int32](val T) uint32 {
	if val < 0 {
		return (uint32(val&0b011111111111111111100) | 1<<20) << 3
	}
	return uint32(val&0b111111111111111111100) << 3
}
func immlo[T ~uint32 | ~int32](val T) uint32 {
	return uint32(val&0b11) << 29
}

func imm16[T ~uint16 | ~int16](val T) uint32 {
	// For uint16, we just use the value directly
	// For int16, we convert to uint16 which will preserve the bit pattern
	return uint32(uint16(val))
}
