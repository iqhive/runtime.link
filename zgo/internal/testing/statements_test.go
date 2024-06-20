package main

import "testing"

func TestBlock(t *testing.T) {
	{
		// TestBlock
	}
}

func TestDefer(t *testing.T) {
	defer println("Hello, World!")
}

func TestFor(t *testing.T) {
	for i := 0; i < 10; i++ {
		println(i)
	}
}

func TestIf(t *testing.T) {
	if true {
		println("true")
	} else {
		println("false")
	}
}

func TestGo(t *testing.T) {
	f := func() {
		println("running in a goroutine!")
	}
	go f()
}
