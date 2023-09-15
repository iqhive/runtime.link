package std_test

import (
	"testing"

	"runtime.link/std"
)

func TestBox(t *testing.T) {
	type StringOrInt std.Variant[any, struct {
		String std.Vary[StringOrInt, string]
		Number std.Vary[StringOrInt, int]
	}]
	var StringOrInts = new(StringOrInt).Values()

	var val StringOrInt = StringOrInts.Number.As(22)

	if val.String() != "22" {
		t.Fatal("unexpected value")
	}
	if StringOrInts.Number.Get(val) != 22 {
		t.Fatal("unexpected value")
	}

	switch std.KindOf(val) {
	case StringOrInts.String.Kind:
		t.Fatal("unexpected value")
	case StringOrInts.Number.Kind:

	default:
		t.Fatal("unexpected value")
	}

	val = StringOrInts.String.As("hello")

	if val.String() != "hello" {
		t.Fatal("unexpected value")
	}
	if StringOrInts.String.Get(val) != "hello" {
		t.Fatal("unexpected value")
	}

}
