package lib_test

import (
	"fmt"
	"math"
	"testing"

	"runtime.link/api"
	link "runtime.link/api/link"
	"runtime.link/lib"
)

func TestLibC(t *testing.T) {
	t.Skip("TODO")
	return
	var libc = api.Import[lib.C](link.API, "", link.CGO)
	fmt.Println(libc.ASCII.IsAlpha('a'))
	fmt.Println(libc.ASCII.IsAlpha('0'))
	fmt.Println(libc.Math.Sqrt(2))
	fmt.Println(libc.Math.Abs(-2))

	if err := libc.IO.PutString("Hello, World!"); err != nil {
		t.Error(err)
	}
}

func BenchmarkSqrt(b *testing.B) {
	var libc = api.Import[lib.C](link.API, "", link.CGO)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		libc.Math.Sqrt(2)
	}
}

func BenchmarkGO(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		math.Sqrt(2)
	}
}
