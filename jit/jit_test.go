package jit_test

import (
	"testing"

	"runtime.link/jit"
)

func TestJIT(t *testing.T) {
	add, err := jit.Make[func(uint, uint) uint](func(asm jit.Assembly, args []jit.Value) ([]jit.Value, error) {
		return []jit.Value{asm.Add(args[0], args[1])}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if sum := add(1, 2); sum != 3 {
		t.Fatalf("expected 3, got %d", sum)
	}
}
