package jit

import (
	"fmt"
	"syscall"
)

func compile(code []byte) ([]byte, error) {
	exec, err := syscall.Mmap(
		-1,
		0,
		len(code),
		syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE,
	)
	if err != nil {
		return nil, fmt.Errorf("mmap: %w", err)
	}
	copy(exec, code)
	if err := syscall.Mprotect(exec, syscall.PROT_EXEC); err != nil {
		return nil, fmt.Errorf("mprotect: %w", err)
	}
	return exec, nil
}
