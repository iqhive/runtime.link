package qty_test

import (
	"fmt"
	"testing"

	"runtime.link/qty"
	"runtime.link/qty/std/metric"
	"runtime.link/qty/std/metric/prefix/kilo"
	"runtime.link/qty/std/physical"
)

func TestQuantity(t *testing.T) {
	var mass qty.Int[physical.Mass, metric.Grams] = 22
	mass.Set(kilo.Grams(2))
	fmt.Println(mass)
}
