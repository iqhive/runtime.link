package main

import "testing"

func Add(a, b int) int {
	return a + b
}

func TestAdd(t *testing.T) {
	result := Add(2, 3)
	if result != 5 {
		t.FailNow()
	}
}

func VariadicFunction(nums ...int) int {
	sum := 0
	for _, num := range nums {
		sum += num
	}
	return sum
}

func TestVariadicFunction(t *testing.T) {
	result := VariadicFunction(1, 2, 3, 4)
	if result != 10 {
		t.FailNow()
	}
}
