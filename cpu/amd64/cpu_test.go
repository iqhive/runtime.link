package amd64_test

import (
	"fmt"
	"testing"

	"runtime.link/cpu/amd64"
)

func TestAddWithCarry(t *testing.T) {
	fn, err := amd64.Compile[func(uint64) uint64](
		amd64.AddWithCarry(amd64.RAX, amd64.RAX),
		amd64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(fn(2))
	if fn(1) != 1 {
		t.Fatal("failed")
	}
}
