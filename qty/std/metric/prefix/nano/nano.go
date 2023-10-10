// Package nano provides provides units based around the SI 'nano' metric prefix.
package nano

import (
	"fmt"
	"math/big"
	"time"

	"runtime.link/qty"
	"runtime.link/qty/std/measures"
)

// Time counts a number of nanoseconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in nanoseconds.
type Seconds float64

// SecondsFrom converts a quantity of time to nanoseconds.
func SecondsFrom(duration qty.That[measures.Duration]) Seconds {
	unit, factor, _ := duration.Quantity()
	secs := unit.Seconds.Mul(unit.Seconds, factor)
	secs = secs.Quo(secs, big.NewFloat(1e9))
	f64, _ := secs.Float64()
	return Seconds(f64)
}

// String implements fmt.Stringer.
func (ns Seconds) String() string { return fmt.Sprintf("%gns", ns) }

// Quantity implements qty.That[measures.Duration].
func (ns Seconds) Quantity() (measures.Duration, *big.Float, string) {
	return measures.Duration{Seconds: big.NewFloat(1e-9)}, big.NewFloat(float64(ns)), "ns"
}

// Metres counts a distance in nanometres.
type (
	Metres float64
	Meters = Metres
)

// MetresFrom converts a quantity of distance to nanometres.
func MetresFrom(distance qty.That[measures.Distance]) Metres {
	unit, factor, _ := distance.Quantity()
	metres := unit.Metres.Mul(unit.Metres, factor)
	metres = metres.Quo(metres, big.NewFloat(1e9))
	f64, _ := metres.Float64()
	return Metres(f64)
}

// MetersFrom is an alias to MetresFrom.
func MetersFrom(distance qty.That[measures.Distance]) Meters { return MetresFrom(distance) }

// String implements fmt.Stringer.
func (nm Metres) String() string { return fmt.Sprintf("%gnm", nm) }

// Quantity implements qty.That[measures.Distance].
func (nm Metres) Quantity() (measures.Distance, *big.Float, string) {
	return measures.Distance{Metres: big.NewFloat(1e-9)}, big.NewFloat(float64(nm)), "nm"
}

// Grams counts a mass in nanograms.
type Grams float64

// GramsFrom converts a quantity of mass to nanograms.
func GramsFrom(mass qty.That[measures.Mass]) Grams {
	unit, factor, _ := mass.Quantity()
	grams := unit.Grams.Mul(unit.Grams, factor)
	grams = grams.Quo(grams, big.NewFloat(1e9))
	f64, _ := grams.Float64()
	return Grams(f64)
}

// String implements fmt.Stringer.
func (ng Grams) String() string { return fmt.Sprintf("%gng", ng) }

// Quantity implements qty.That[measures.Mass].
func (ng Grams) Quantity() (measures.Mass, *big.Float, string) {
	return measures.Mass{Grams: big.NewFloat(1e-9)}, big.NewFloat(float64(ng)), "ng"
}

// Amps counts a current in nanoamps.
type Amps float64

// AmpsFrom converts a quantity of current to nanoamps.
func AmpsFrom(current qty.That[measures.Current]) Amps {
	unit, factor, _ := current.Quantity()
	amps := unit.Amps.Mul(unit.Amps, factor)
	amps = amps.Quo(amps, big.NewFloat(1e9))
	f64, _ := amps.Float64()
	return Amps(f64)
}

// String implements fmt.Stringer.
func (na Amps) String() string { return fmt.Sprintf("%gnA", na) }

// Quantity implements qty.That[measures.Current].
func (na Amps) Quantity() (measures.Current, *big.Float, string) {
	return measures.Current{Amps: big.NewFloat(1e-9)}, big.NewFloat(float64(na)), "nA"
}
