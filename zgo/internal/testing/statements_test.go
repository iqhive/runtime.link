package main

import "testing"

func TestBlock(t *testing.T) {
	{
		// TestBlock
	}
}

func TestDefer(t *testing.T) {
	f := func() {}
	defer f()
}

func TestFor(t *testing.T) {
	var accum int
	for i := 0; i < 10; i++ {
		accum += i
	}
	if accum != 45 {
		t.FailNow()
	}
}

func TestIf(t *testing.T) {
	if true {
	} else {
		t.FailNow()
	}
}

func TestGo(t *testing.T) {
	f := func() {}
	go f()
}
