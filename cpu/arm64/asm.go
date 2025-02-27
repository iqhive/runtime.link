package arm64

import "encoding/binary"

type Assembly interface {
	AssembleArm64([]byte) []byte
}

type literal uint32

const (
	ret = literal(0xD65F03C0)
)

func (l literal) AssembleArm64(buf []byte) []byte {
	return binary.LittleEndian.AppendUint32(buf, uint32(l))
}

type instruction struct {
	op         uint32
	rd, rn, rm Register
}

func (i instruction) AssembleArm64(buf []byte) []byte {
	instruction := uint32(i.op)
	instruction |= uint32(i.rn&0x1F) << 5
	instruction |= uint32(i.rd & 0x1F)
	instruction |= uint32(i.rm&0x1F) << 16
	return binary.LittleEndian.AppendUint32(buf, instruction)
}
