package main

import "testing"

func TestShadowing(t *testing.T) {
	/*
		Shadowing is not supported in zig but it is in Go, so the compiler
		needs to be able to handle this case.
	*/
	{
		var x int = 1
		{
			var x int = 2
			if x != 2 {
				t.FailNow()
			}
		}
		if x != 1 {
			t.FailNow()
		}
	}
}
