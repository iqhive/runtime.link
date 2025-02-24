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

func TestBitwiseAndInstruction(t *testing.T) {
	fn, err := amd64.Compile[func(uint64, uint64) uint64](
		amd64.BitwiseAnd(amd64.RAX, amd64.RBX),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fn(0xFF00, 0x0FF0) != 0x0F00 {
		t.Fatal("unexpected value")
	}
}

func TestBitwiseOrInstruction(t *testing.T) {
	fn, err := amd64.Compile[func(uint64, uint64) uint64](
		amd64.BitwiseOr(amd64.RAX, amd64.RBX),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fn(0xF000, 0x0F00) != 0xFF00 {
		t.Fatal("unexpected value")
	}
}

func TestBitwiseXorInstruction(t *testing.T) {
	fn, err := amd64.Compile[func(uint64, uint64) uint64](
		amd64.BitwiseXor(amd64.RAX, amd64.RBX),
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
		amd64.BitwiseAnd(amd64.EAX, amd64.Imm32(0x0F0F)),
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
		amd64.BitwiseOr(amd64.EAX, amd64.Imm32(0x0F0F)),
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
		amd64.BitwiseXor(amd64.EAX, amd64.Imm32(0x0F0F)),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if xorFn(0xFF00) != 0xF00F {
		t.Fatal("unexpected XOR result")
	}
}

func TestMemoryBitwiseOperations(t *testing.T) {
	// Test AND with memory operand
	andFn, err := amd64.Compile[func(*uint32)](
		amd64.MemoryBitwiseAnd(amd64.EAX.AsPointer(), amd64.Imm32(0x0F0F)),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	var andValue uint32 = 0xFF00
	andFn(&andValue)
	if andValue != 0x0F00 {
		t.Fatal("unexpected AND result:", andValue)
	}

	// Test OR with memory operand
	orFn, err := amd64.Compile[func(*uint32)](
		amd64.MemoryBitwiseOr(amd64.EAX.AsPointer(), amd64.Imm32(0x0F0F)),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	var orValue uint32 = 0xF000
	orFn(&orValue)
	if orValue != 0xFF0F {
		t.Fatal("unexpected OR result:", orValue)
	}

	// Test XOR with memory operand
	xorFn, err := amd64.Compile[func(*uint32)](
		amd64.MemoryBitwiseXor(amd64.EAX.AsPointer(), amd64.Imm32(0x0F0F)),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	var xorValue uint32 = 0xFF00
	xorFn(&xorValue)
	if xorValue != 0xF00F {
		t.Fatal("unexpected XOR result:", xorValue)
	}
}
