package bit

import "unsafe"

type Array struct {
	buffer complex128
	length int
	memory unsafe.Pointer
}

func (arr Array) Len() int { return arr.length }

func Bytes(data ...byte) Array {
	if len(data) > 16 {
		var temp = make([]byte, len(data))
		copy(temp, data)
		return Array{
			length: len(data) * 8,
			memory: unsafe.Pointer(&temp[0]),
		}
	}
	var array Array
	array.length = len(data) * 8
	copy((*[16]byte)(unsafe.Pointer(&array))[:], data)
	return array
}

func String(data string) Array {
	if len(data) > 16 {
		var temp = make([]byte, len(data))
		copy(temp, data)
		return Array{
			length: len(data) * 8,
			memory: unsafe.Pointer(&temp[0]),
		}
	}
	var array Array
	array.length = len(data) * 8
	copy((*[16]byte)(unsafe.Pointer(&array))[:], data)
	return array
}

func (arr Array) Bytes() []byte {
	if arr.memory != nil {
		return unsafe.Slice((*byte)(arr.memory), arr.length/8)
	}
	return (*[16]byte)(unsafe.Pointer(&arr))[0 : arr.length/8]
}

type Writer interface {
	WriteBits(Array) (int, error)
}

type Reader interface {
	ReadBits(int) (Array, error)
}
