// Package micro provides a standard way to represent a millionth of a SI unit.
package micro

import (
	"fmt"
	"math/big"
	"time"

	"runtime.link/qty"
	"runtime.link/qty/std/physical"
)

// Time counts a number of microseconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in microseconds.
type Seconds float64

// SecondsFrom converts a quantity of time to microseconds.
func SecondsFrom(duration qty.Measures[physical.Duration]) Seconds {
	unit, factor, _ := duration.Quantity()
	secs := unit.Seconds.Mul(unit.Seconds, factor)
	secs = secs.Quo(secs, big.NewFloat(1e6))
	f64, _ := secs.Float64()
	return Seconds(f64)
}

// String implements fmt.Stringer.
func (μs Seconds) String() string { return fmt.Sprintf("%gμs", μs) }

// Quantity implements qty.Measures[physical.Duration].
func (μs Seconds) Quantity() (physical.Duration, *big.Float, string) {
	return physical.Duration{Seconds: big.NewFloat(1e-6)}, big.NewFloat(float64(μs)), "μs"
}

// Metres counts a distance in micrometres.
type (
	Metres float64
	Meters = Metres
)

// MetresFrom converts a quantity of distance to micrometres.
func MetresFrom(distance qty.Measures[physical.Distance]) Metres {
	unit, factor, _ := distance.Quantity()
	metres := unit.Metres.Mul(unit.Metres, factor)
	metres = metres.Quo(metres, big.NewFloat(1e6))
	f64, _ := metres.Float64()
	return Metres(f64)
}

// MetersFrom is an alias to MetresFrom.
func MetersFrom(distance qty.Measures[physical.Distance]) Meters { return MetresFrom(distance) }

// String implements fmt.Stringer.
func (μm Metres) String() string { return fmt.Sprintf("%gμm", μm) }

// Metres implements qty.Measures[physical.Distance].
func (μm Metres) Quantity() (physical.Distance, *big.Float, string) {
	return physical.Distance{Metres: big.NewFloat(1e-6)}, big.NewFloat(float64(μm)), "μm"
}

// Grams counts a mass in micrograms.
type Grams float64

// GramsFrom converts a quantity of mass to micrograms.
func GramsFrom(mass qty.Measures[physical.Mass]) Grams {
	unit, factor, _ := mass.Quantity()
	grams := unit.Grams.Mul(unit.Grams, factor)
	grams = grams.Quo(grams, big.NewFloat(1e6))
	f64, _ := grams.Float64()
	return Grams(f64)
}

// String implements fmt.Stringer.
func (μg Grams) String() string { return fmt.Sprintf("%gμg", μg) }

// Quantity implements qty.Measures[physical.Mass].
func (μg Grams) Quantity() (physical.Mass, *big.Float, string) {
	return physical.Mass{Grams: big.NewFloat(1e-6)}, big.NewFloat(float64(μg)), "μg"
}

// Amps counts a current in microamps.
type Amps float64

// AmpsFrom converts a quantity of current to microamps.
func AmpsFrom(current qty.Measures[physical.Current]) Amps {
	unit, factor, _ := current.Quantity()
	amps := unit.Amps.Mul(unit.Amps, factor)
	amps = amps.Quo(amps, big.NewFloat(1e6))
	f64, _ := amps.Float64()
	return Amps(f64)
}

// String implements fmt.Stringer.
func (μA Amps) String() string { return fmt.Sprintf("%gμA", μA) }

// Quantity implements qty.Measures[physical.Current].
func (μA Amps) Quantity() (physical.Current, *big.Float, string) {
	return physical.Current{Amps: big.NewFloat(1e-6)}, big.NewFloat(float64(μA)), "μA"
}
