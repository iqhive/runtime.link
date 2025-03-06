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
		ABS:  func(dst, src X) error { return asm.write(0b101101011000000001<<13 | rd(dst) | rn(src)) },
		ADC:  func(dst, a, b X) error { return asm.write(0b1001101<<25 | rd(dst) | rn(a) | rm(b)) },
		ADCS: func(dst, a, b X) error { return asm.write(0b1011101<<25 | rd(dst) | rn(a) | rm(b)) },
		ADD: NewExtendedImmediateShifted(
			func(dst, src1, src2 X, extension RegisterExtension, amount Uint3) error {
				return asm.write(0b10001011001<<21 | rd(dst) | rn(src1) | rm(src2) | imm3(amount)<<10 | imm3(extension)<<13)
			},
			func(dst, src X, immediate Uint12) error {
				return asm.write(0b10010001<<24 | rd(dst) | rn(src) | imm12(immediate)<<10)
			},
			func(dst, a, b X, shift Shift, amount Uint6) error {
				return asm.write(0b10001011<<24 | rd(dst) | rn(a) | imm6(amount)<<10 | imm2(shift)<<22 | rm(b))
			},
		),
		ADDG: func(dst, src X, offset Int6, tag_offset Int4) error {
			return asm.write(0b100100011<<22 | rd(dst) | rn(src) | imm6(offset)<<16 | imm4(tag_offset)<<10)
		},
		ADDPT: func(dst, ptr, offset X, shift_amount Uint3) error {
			return asm.write(0b1001101<<25 | rd(dst) | rn(ptr) | imm3(shift_amount)<<10 | rm(offset))
		},
		ADDS: NewExtendedImmediateShifted(
			func(dst, src1, src2 X, extension RegisterExtension, amount Uint3) error {
				return asm.write(0b10101011001<<21 | rd(dst) | rn(src1) | rm(src2) | imm3(amount)<<10 | imm3(extension)<<13)
			},
			func(dst, src X, immediate Uint12) error {
				return asm.write(0b10110001<<24 | rd(dst) | rn(src) | imm12(immediate)<<10)
			},
			func(dst, a, b X, shift Shift, amount Uint6) error {
				return asm.write(0b10101011<<24 | rd(dst) | rn(a) | imm6(amount)<<10 | imm2(shift)<<22 | rm(b))
			},
		),
		CMPs: func(src1, src2 X, shift Shift, amount Uint6) error {
			return asm.write(0b11101011<<24 | rn(src1) | rm(src2) | imm6(amount)<<10 | uint32(shift)<<22)
		},
		CSET: func(dst X, condition Condition) error {
			return asm.write(0b100110101001111100000111111<<5 | rd(dst) | imm4(condition)<<12)
		},
		RET: func(lnk X) error { return asm.write(0b1101011001011111<<16 | rn(lnk)) },
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

func rd(reg X) uint32 { return uint32(reg&0b11111) << 0 }
func rn(reg X) uint32 { return uint32(reg&0b11111) << 5 }
func rm(reg X) uint32 { return uint32(reg&0b11111) << 16 }

func imm2[T ~uint8 | ~int8](val T) uint32 {
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
func imm6[T ~uint8 | ~int8](val T) uint32 {
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
