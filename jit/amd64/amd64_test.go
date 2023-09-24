package amd64_test

import (
	"runtime"
	"testing"

	"runtime.link/jit"
	"runtime.link/jit/amd64"
)

func TestJIT(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("skipping test on non-amd64 platform")
		return
	}
	var (
		src = jit.New[amd64.InstructionSet]()
		asm = src.Assemble
	)

	symAdd := src.Symbol()
	asm.Math.Add(amd64.RAX, amd64.RBX)
	asm.Return()

	if err := src.Compile(); err != nil {
		t.Fatal(err)
	}

	add := jit.Make[func(uint, uint) uint](symAdd)
	if add(1, 2) != 3 {
		t.Fatal("add(1, 2) != 3")
	}
}
