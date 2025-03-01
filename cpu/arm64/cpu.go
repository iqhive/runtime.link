//go:build arm64

package arm64

import (
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"syscall"
	"unsafe"
)

func Compile[F any](asm ...Instruction) (fn F, err error) {
	// Validate that F is a function type
	if reflect.TypeFor[F]().Kind() != reflect.Func {
		return fn, errors.New("expected function type")
	}
	// Assemble the instructions into a buffer
	var buf []byte
	for _, a := range asm {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(a))
	}
	// Ensure buffer length is a multiple of 4 (ARM64 instruction alignment)
	if len(buf)%4 != 0 {
		return fn, errors.New("instruction buffer must be 4-byte aligned")
	}
	// Map memory as writable first (macOS W^X requires separate steps)
	mem, err := syscall.Mmap(-1, 0, len(buf),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_PRIVATE|syscall.MAP_ANON)
	if err != nil {
		return fn, fmt.Errorf("mmap failed: %v", err)
	}
	// Copy the assembled instructions into the mapped memory
	copy(mem, buf)
	// Change permissions to executable (remove write, add exec)
	err = syscall.Mprotect(mem, syscall.PROT_READ|syscall.PROT_EXEC)
	if err != nil {
		return fn, fmt.Errorf("mprotect failed: %v", err)
	}
	pc := &mem[0]
	ptr := &pc
	return *(*F)(unsafe.Pointer(&ptr)), nil
}
