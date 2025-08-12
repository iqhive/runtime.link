package main

import "testing"

func my_function() *int {
	var x int = 42
	return &x
}

func TestReturnValues(t *testing.T) {
	result := my_function()
	if result == nil {
		t.FailNow()
	}
	if *result != 42 {
		t.FailNow()
	}
}
