package lib_test

import (
	"fmt"
	"testing"

	"runtime.link/lib"
)

func TestLibC(t *testing.T) {
	var libc = lib.Import[lib.C]()
	fmt.Println(libc.Math.Sqrt(2))

	if err := libc.IO.PutString("Hello, World!"); err != nil {
		t.Error(err)
	}

	/*if err := libc.IO.PutString("Hello, World!"); err != nil {
		t.Error(err)
	}
	if err := libc.IO.PutString("Hello, World!"); err != nil {
		t.Error(err)
	}*/
}

func BenchmarkSqrt(b *testing.B) {
	var libc = lib.Import[lib.C]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		libc.Math.Sqrt(1)
	}
}
