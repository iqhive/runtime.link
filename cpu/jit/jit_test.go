package jit_test

import (
	"testing"

	"runtime.link/cpu/jit"
)

func TestJIT(t *testing.T) {
	src := new(jit.Program)
	src.CompileOnce = true
	add := jit.Make[func(uint, uint) uint](&src, func(a, b jit.Uint) jit.Uint {
		return jit.Add(src, a, b)
	})
	if sum := add(1, 2); sum != 3 {
		t.Fatalf("expected 3, got %d", sum)
	}
}
