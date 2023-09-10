package std_test

import (
	"fmt"
	"testing"

	"runtime.link/std"
)

func TestBox(t *testing.T) {
	type StringOrInt std.Variant[any, struct {
		String std.Vary[StringOrInt, string]
		Number std.Vary[StringOrInt, int]
	}]
	var StringOrInts = new(StringOrInt).Values()

	var val StringOrInt
	val = StringOrInts.Number.As(22)

	fmt.Println(val)
}
