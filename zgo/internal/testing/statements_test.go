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

func TestBreak(t *testing.T) {
	var i int
	for i = 0; i < 10; i++ {
		if i == 5 {
			break
		}
	}
	if i != 5 {
		t.FailNow()
	}

original:
	for range 10 {
		for range 10 {
			break original
		}
	}
}

func TestContinue(t *testing.T) {
	var accum int
	for i := 0; i < 10; i++ {
		if i == 5 {
			continue
		}
		accum += i
	}
	if accum != 40 {
		t.FailNow()
	}

	var i int
original:
	for i = 0; i < 10; i++ {
		for j := range 2 {
			if j == 1 {
				continue original
			}
			i++
		}
	}
	if i != 10 {
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
