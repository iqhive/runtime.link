// Package kilo provides a standard way to represent thousands for a SI unit.
package kilo

import (
	"fmt"
	"math/big"

	"runtime.link/qty"
	"runtime.link/qty/std/physical"
)

// Metres counts a distance in kilometres.
type (
	Metres float64
	Meters = Metres
)

// MetresFrom converts a quantity of distance to kilometres.
func MetresFrom(distance qty.Measures[physical.Distance]) Metres {
	unit, factor, _ := distance.Quantity()
	metres := unit.Metres.Mul(unit.Metres, factor)
	metres = metres.Quo(metres, big.NewFloat(1e3))
	f64, _ := metres.Float64()
	return Metres(f64)
}

// MetersFrom is an alias to MetresFrom.
func MetersFrom(distance qty.Measures[physical.Distance]) Meters { return MetresFrom(distance) }

// String implements fmt.Stringer.
func (km Metres) String() string { return fmt.Sprintf("%gkm", km) }

// Quantity implements [qty.Measures[physical.Distance]]
func (km Metres) Quantity() (physical.Distance, *big.Float, string) {
	return physical.Distance{Metres: big.NewFloat(1e3)}, big.NewFloat(float64(km)), "km"
}

// Grams counts a mass in kilograms.
type Grams float64

// GramsFrom converts a quantity of mass to kilograms.
func GramsFrom(mass qty.Measures[physical.Mass]) Grams {
	unit, factor, _ := mass.Quantity()
	grams := unit.Grams.Mul(unit.Grams, factor)
	grams = grams.Quo(grams, big.NewFloat(1e3))
	f64, _ := grams.Float64()
	return Grams(f64)
}

// String implements fmt.Stringer.
func (kg Grams) String() string { return fmt.Sprintf("%gkg", kg) }

// Grams implements qty.Mass.
func (kg Grams) Quantity() (physical.Mass, *big.Float, string) {
	return physical.Mass{Grams: big.NewFloat(1e3)}, big.NewFloat(float64(kg)), "kg"
}

// Bytes counts a digital storage in kilobytes.
type Bytes uint64

// BytesFrom converts a quantity of digital storage to kilobytes.
func BytesFrom(information qty.Measures[physical.Information]) Bytes {
	unit, factor, _ := information.Quantity()
	bytes := unit.Bits.Mul(unit.Bits, factor)
	bytes = bytes.Quo(bytes, big.NewFloat(8e3))
	u64, _ := bytes.Uint64()
	return Bytes(u64)
}

// String implements fmt.Stringer.
func (kB Bytes) String() string { return fmt.Sprintf("%dkB", kB) }

// Quantity implements [qty.Measures[physical.Information]]
func (kB Bytes) Quantity() (physical.Information, *big.Float, string) {
	var f big.Float
	f.SetUint64(uint64(kB))
	return physical.Information{Bits: big.NewFloat(8e3)}, &f, "kB"
}

// Bits counts a digital storage in kilobits.
type Bits uint64

// BitsFrom converts a quantity of digital storage to kilobits.
func BitsFrom(information qty.Measures[physical.Information]) Bits {
	unit, factor, _ := information.Quantity()
	bits := unit.Bits.Mul(unit.Bits, factor)
	bits = bits.Quo(bits, big.NewFloat(1e3))
	u64, _ := bits.Uint64()
	return Bits(u64)
}

// String implements fmt.Stringer.
func (kb Bits) String() string { return fmt.Sprintf("%dkb", kb) }

// Quantity implements [qty.Measures[physical.Information]]
func (kb Bits) Quantity() (physical.Information, *big.Float, string) {
	var f big.Float
	f.SetUint64(uint64(kb))
	return physical.Information{Bits: big.NewFloat(1e3)}, &f, "kb"
}
