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
