package main

import "testing"

func TestIfElse(t *testing.T) {
	a := 10
	if a > 5 {
	} else {
		t.FailNow()
	}
}

func TestForLoop(t *testing.T) {
	sum := 0
	for i := 1; i <= 5; i++ {
		sum += i
	}
	if sum != 15 {
		t.FailNow()
	}
}

func TestSwitch(t *testing.T) {
	a := 2
	switch a {
	case 1:
		t.FailNow()
	case 2:
	default:
		t.FailNow()
	}
}
