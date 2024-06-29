package main

import "testing"

func TestBasicTypes(t *testing.T) {
	var a int = 10
	var b float64 = 20.5
	var c string = "hello"
	var d bool = true

	if a != 10 {
		t.FailNow()
	}
	if b != 20.5 {
		t.FailNow()
	}
	if c != "hello" {
		t.FailNow()
	}
	if !d {
		t.FailNow()
	}
}
