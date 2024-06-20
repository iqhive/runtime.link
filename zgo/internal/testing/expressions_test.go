package main

import "testing"

func TestExpressionFunction(t *testing.T) {
	var add = func(a, b int) int {
		return a + b
	}
	if add(1, 2) != 3 {
		t.FailNow()
	}
}
