package main

import "testing"

func TestArrays(t *testing.T) {
	arr := [3]int{1, 2, 3}
	if arr[0] != 1 || arr[1] != 2 || arr[2] != 3 {
		t.FailNow()
	}
}

func TestSlices(t *testing.T) {
	slice := []int{1, 2, 3}
	slice = append(slice, 4)
	if len(slice) != 4 || slice[3] != 4 {
		t.FailNow()
	}
}

func TestMaps(t *testing.T) {
	m := map[string]int{"one": 1, "two": 2}
	if m["one"] != 1 || m["two"] != 2 {
		t.FailNow()
	}
}

type Person struct {
	Name string
	Age  int
}

func TestStructs(t *testing.T) {
	p := Person{Name: "Alice", Age: 30}
	if p.Name != "Alice" || p.Age != 30 {
		t.FailNow()
	}
}
