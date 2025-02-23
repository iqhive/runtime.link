package amd64

import (
	"encoding"
	"errors"
	"reflect"
	"syscall"
	"unsafe"
)

func Compile[F any](asm ...encoding.BinaryAppender) (fn F, err error) {
	if reflect.TypeFor[F]().Kind() != reflect.Func {
		return [1]F{}[0], errors.New("expected function type")
	}
	var buf []byte
	for _, a := range asm {
		buf, err = a.AppendBinary(buf)
		if err != nil {
			return [1]F{}[0], err
		}
	}
	mem, err := syscall.Mmap(-1, 0, len(buf),
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC,
		syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS)
	if err != nil {
		return [1]F{}[0], err
	}
	copy(mem, buf)
	ptr := &mem[0]
	nxt := &ptr
	return *(*F)(unsafe.Pointer(&nxt)), nil
}
