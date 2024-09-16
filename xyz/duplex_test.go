package xyz_test

import (
	"fmt"
	"testing"

	"runtime.link/xyz"
)

func TestDuplex(t *testing.T) {
	var a = xyz.Duplex[float64]{5, 2, 1}
	fmt.Println(a)
	fmt.Println(a.Values())
}
