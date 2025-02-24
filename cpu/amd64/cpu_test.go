//go:build amd64

package amd64_test

import (
	"testing"

	"runtime.link/cpu/amd64"
)

func TestAddAsmCode(t *testing.T) {
	fn, err := amd64.Compile[func(uint32, uint32) uint32](
		amd64.Add(amd64.EAX, amd64.EBX),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fn(1, 2) != 3 {
		t.Fatal("unexpected value")
	}
}

func TestMemoryAddWithCarry(t *testing.T) {
	fn, err := amd64.Compile[func(*uint32)](
		amd64.MemoryAddWithCarry(amd64.EAX.AsPointer(), amd64.Imm32(2)),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	var value uint32 = 1
	fn(&value)
	if value != 3 {
		t.Fatal("unexpected value:", value)
	}
}

func TestAddWithCarry2(t *testing.T) {
	fn, err := amd64.Compile[func(uint32) uint32](
		amd64.AddWithCarry(amd64.EAX, amd64.Imm32(2)),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fn(1) != 3 {
		t.Fatal("unexpected value")
	}
}

func TestAndInstruction(t *testing.T) {
	fn, err := amd64.Compile[func(uint64, uint64) uint64](
		amd64.And(amd64.RAX, amd64.RBX),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fn(0xFF00, 0x0FF0) != 0x0F00 {
		t.Fatal("unexpected value")
	}
}

func TestOrInstruction(t *testing.T) {
	fn, err := amd64.Compile[func(uint64, uint64) uint64](
		amd64.Or(amd64.RAX, amd64.RBX),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fn(0xF000, 0x0F00) != 0xFF00 {
		t.Fatal("unexpected value")
	}
}

func TestXorInstruction(t *testing.T) {
	fn, err := amd64.Compile[func(uint64, uint64) uint64](
		amd64.Xor(amd64.RAX, amd64.RBX),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fn(0xFF00, 0x0FF0) != 0xF0F0 {
		t.Fatal("unexpected value")
	}
}

func TestBitInstructionsWithImmediate(t *testing.T) {
	// Test AND with immediate
	andFn, err := amd64.Compile[func(uint32) uint32](
		amd64.And(amd64.EAX, amd64.Imm32(0x0F0F)),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if andFn(0xFF00) != 0x0F00 {
		t.Fatal("unexpected AND result")
	}

	// Test OR with immediate
	orFn, err := amd64.Compile[func(uint32) uint32](
		amd64.Or(amd64.EAX, amd64.Imm32(0x0F0F)),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if orFn(0xF000) != 0xFF0F {
		t.Fatal("unexpected OR result")
	}

	// Test XOR with immediate
	xorFn, err := amd64.Compile[func(uint32) uint32](
		amd64.Xor(amd64.EAX, amd64.Imm32(0x0F0F)),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if xorFn(0xFF00) != 0xF00F {
		t.Fatal("unexpected XOR result")
	}
}
