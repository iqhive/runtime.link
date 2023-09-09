package box_test

import (
	"fmt"
	"testing"

	"runtime.link/box"
)

func TestBox(t *testing.T) {
	type StringOrInt box.Variant[any, struct {
		String box.Vary[StringOrInt, string]
		Number box.Vary[StringOrInt, int]
	}]
	var StringOrInts = new(StringOrInt).Values()

	var val StringOrInt
	val = StringOrInts.Number.As(22)

	fmt.Println(val)
}
