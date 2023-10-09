// Package centi provides standard types to represent a hundredth of a SI unit.
package centi

import (
	"fmt"
	"math/big"

	"runtime.link/qty"
	"runtime.link/qty/std/physical"
)

// Metres counts a distance in centimetres.
type (
	Metres float64
	Meters = Metres
)

// MetresFrom converts a quantity of distance to centimetres.
func MetresFrom(distance qty.Measures[physical.Distance]) Metres {
	unit, factor, _ := distance.Quantity()
	metres := unit.Metres.Mul(unit.Metres, factor)
	metres = metres.Quo(metres, big.NewFloat(1e2))
	f64, _ := metres.Float64()
	return Metres(f64)
}

// MetersFrom is an alias to MetresFrom.
func MetersFrom(distance qty.Measures[physical.Distance]) Meters { return MetresFrom(distance) }

// String implements fmt.Stringer.
func (cm Metres) String() string { return fmt.Sprintf("%gcm", cm) }

// Quantity implements [qty.Measures[physical.Distance]]
func (cm Metres) Quantity() (physical.Distance, *big.Float, string) {
	return physical.Distance{Metres: big.NewFloat(1e-2)}, big.NewFloat(float64(cm)), "cm"
}
