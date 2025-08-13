package main

import "testing"

func return_value_escapes() *int {
	var x int = 42
	return &x
}

func return_value_noescape(p *int) *int {
	return p
}

var globalInt *int

func return_value_escapes_through_global(p *int) *int {
	globalInt = p
	return p
}

func test_globals(t *testing.T) {
	var pp int = 33
	result3 := return_value_escapes_through_global(&pp)
	if result3 == nil {
		t.FailNow()
	}
	if *result3 != 33 {
		t.FailNow()
	}
}

func TestReturnValues(t *testing.T) {
	test_globals(t)

	result := return_value_escapes()
	if result == nil {
		t.FailNow()
	}
	if *result != 42 {
		t.FailNow()
	}

	var p int = 22
	result2 := return_value_noescape(&p)
	if result2 == nil {
		t.FailNow()
	}
	if *result2 != 22 {
		t.FailNow()
	}

	if *globalInt != 33 {
		t.FailNow()
	}
}
