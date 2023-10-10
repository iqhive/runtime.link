package qty_test

import (
	"testing"

	"runtime.link/qty"
	"runtime.link/qty/std/binary/prefix/mebi"
	"runtime.link/qty/std/metric"
	"runtime.link/qty/std/metric/prefix/kilo"
	"runtime.link/qty/std/physical"
)

func TestQuantity(t *testing.T) {
	var mass qty.Int[physical.Mass, metric.Grams] = 22
	mass.Set(kilo.Grams(2))
	if mass != 2000 {
		t.Fatal(mass)
	}

	var data kilo.Bytes = kilo.BytesFrom(mebi.Bytes(1))
	if data != 1048 {
		t.Fatal(data)
	}
}
