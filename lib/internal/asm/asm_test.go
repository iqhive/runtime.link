package asm_test

import (
	"fmt"
	"testing"

	"runtime.link/lib/internal/asm"
	"runtime.link/lib/internal/dll"
)

func TestCall(t *testing.T) {
	libc, err := dll.Open("libSystem.dylib")
	if err != nil {
		t.Fatal(err)
	}
	sqrt, err := dll.Sym(libc, "sqrt")
	if err != nil {
		t.Fatal(err)
	}

	var reg asm.Registers
	reg.PushF64(100.0)
	fmt.Println(asm.Call(sqrt, reg))
}

func BenchmarkSqrt(b *testing.B) {
	libc, err := dll.Open("libSystem.dylib")
	if err != nil {
		b.Fatal(err)
	}
	sqrt, err := dll.Sym(libc, "sqrt")
	if err != nil {
		b.Fatal(err)
	}

	var reg asm.Registers
	reg.PushF64(2.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		asm.Call(sqrt, reg)
	}
}
