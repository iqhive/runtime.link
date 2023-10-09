// Package milli provides a standard way to represent a thousandth of a SI unit.
package milli

import (
	"fmt"
	"math/big"
	"time"

	"runtime.link/qty"
	"runtime.link/qty/std/physical"
)

// Time counts a number of milliseconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in milliseconds.
type Seconds float64

// SecondsFrom converts a quantity of time to milliseconds.
func SecondsFrom(duration qty.Measures[physical.Duration]) Seconds {
	unit, factor, _ := duration.Quantity()
	secs := unit.Seconds.Mul(unit.Seconds, factor)
	secs = secs.Quo(secs, big.NewFloat(1e3))
	f64, _ := secs.Float64()
	return Seconds(f64)
}

// String implements fmt.Stringer.
func (ms Seconds) String() string { return fmt.Sprintf("%gms", ms) }

// Quantity implements qty.Measures[physical.Duration].
func (ms Seconds) Quantity() (physical.Duration, *big.Float, string) {
	return physical.Duration{Seconds: big.NewFloat(1e-3)}, big.NewFloat(float64(ms)), "ms"
}

// Metres counts a distance in millimetres.
type (
	Metres float64
	Meters = Metres
)

// MetresFrom converts a quantity of distance to millimetres.
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
func (mm Metres) String() string { return fmt.Sprintf("%gmm", mm) }

// Quantity implements qty.Measures[physical.Distance].
func (mm Metres) Quantity() (physical.Distance, *big.Float, string) {
	return physical.Distance{Metres: big.NewFloat(1e-3)}, big.NewFloat(float64(mm)), "mm"
}

// Grams counts a mass in milligrams.
type Grams float64

// GramsFrom converts a quantity of mass to milligrams.
func GramsFrom(mass qty.Measures[physical.Mass]) Grams {
	unit, factor, _ := mass.Quantity()
	grams := unit.Grams.Mul(unit.Grams, factor)
	grams = grams.Quo(grams, big.NewFloat(1e3))
	f64, _ := grams.Float64()
	return Grams(f64)
}

// String implements fmt.Stringer.
func (mg Grams) String() string { return fmt.Sprintf("%gmg", mg) }

// Quantity implements qty.Measures[physical.Mass].
func (mg Grams) Quantity() (physical.Mass, *big.Float, string) {
	return physical.Mass{Grams: big.NewFloat(1e-3)}, big.NewFloat(float64(mg)), "mg"
}

// Amps counts a current in milliamps.
type Amps float64

// AmpsFrom converts a quantity of current to milliamps.
func AmpsFrom(current qty.Measures[physical.Current]) Amps {
	unit, factor, _ := current.Quantity()
	amps := unit.Amps.Mul(unit.Amps, factor)
	amps = amps.Quo(amps, big.NewFloat(1e3))
	f64, _ := amps.Float64()
	return Amps(f64)
}

// String implements fmt.Stringer.
func (mA Amps) String() string { return fmt.Sprintf("%gmA", mA) }

// Quantity implements qty.Measures[physical.Current].
func (mA Amps) Quantity() (physical.Current, *big.Float, string) {
	return physical.Current{Amps: big.NewFloat(1e-3)}, big.NewFloat(float64(mA)), "mA"
}
