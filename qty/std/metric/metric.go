package metric

import (
	"fmt"
	"math/big"
	"time"

	"runtime.link/qty"
	"runtime.link/qty/std/physical"
)

// Time counts a number of seconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in seconds.
type Seconds float64

// SecondsFrom converts a quantity of time to seconds.
func SecondsFrom(duration qty.Measures[physical.Duration]) Seconds {
	unit, factor, _ := duration.Quantity()
	secs := unit.Seconds.Mul(unit.Seconds, factor)
	f64, _ := secs.Float64()
	return Seconds(f64)
}

// String implements fmt.Stringer.
func (s Seconds) String() string { return fmt.Sprintf("%gs", s) }

// Seconds implements qty.Time.
func (s Seconds) Quantity() (physical.Duration, *big.Float, string) {
	return physical.Duration{Seconds: big.NewFloat(1)}, big.NewFloat(float64(s)), "s"
}

// Metres counts a distance in metres.
type (
	Metres float64
	Meters = Metres
)

// MetresFrom converts a quantity of distance to metres.
func MetresFrom(distance qty.Measures[physical.Distance]) Metres {
	unit, factor, _ := distance.Quantity()
	secs := unit.Metres.Mul(unit.Metres, factor)
	f64, _ := secs.Float64()
	return Metres(f64)
}

// MetersFrom is an alias to MetresFrom.
func MetersFrom(distance qty.Measures[physical.Distance]) Meters { return MetresFrom(distance) }

// String implements fmt.Stringer.
func (m Metres) String() string { return fmt.Sprintf("%gm", m) }

// Metres implements qty.Distance.
func (m Metres) Quantity() (physical.Distance, *big.Float, string) {
	return physical.Distance{Metres: big.NewFloat(1)}, big.NewFloat(float64(m)), "m"
}

// Grams counts a mass in grams.
type Grams float64

// GramsFrom converts a quantity of mass to kilograms.
func GramsFrom(mass qty.Measures[physical.Mass]) Grams {
	unit, factor, _ := mass.Quantity()
	grams := unit.Grams.Mul(unit.Grams, factor)
	f64, _ := grams.Float64()
	return Grams(f64)
}

// String implements fmt.Stringer.
func (g Grams) String() string { return fmt.Sprintf("%gg", g) }

// Grams implements qty.Mass.
func (g Grams) Quantity() (physical.Mass, *big.Float, string) {
	return physical.Mass{Grams: big.NewFloat(1)}, big.NewFloat(float64(g)), "g"
}

// Amps counts a current in amps.
type Amps float64

// AmpsFrom converts a quantity of current to amps.
func AmpsFrom(current qty.Measures[physical.Current]) Amps {
	unit, factor, _ := current.Quantity()
	amps := unit.Amps.Mul(unit.Amps, factor)
	f64, _ := amps.Float64()
	return Amps(f64)
}

// String implements fmt.Stringer.
func (a Amps) String() string { return fmt.Sprintf("%gA", a) }

// Quantity implements qty.Measures[physical.Current].
func (a Amps) Quantity() (physical.Current, *big.Float, string) {
	return physical.Current{Amps: big.NewFloat(1)}, big.NewFloat(float64(a)), "A"
}

// Kelvin counts a temperature in kelvin.
type Kelvin float64

// KelvinFrom converts a quantity of temperature to kelvin.
func KelvinFrom(temperature qty.Measures[physical.Temperature]) Kelvin {
	unit, factor, _ := temperature.Quantity()
	kelvin := unit.Kelvin.Mul(unit.Kelvin, factor)
	f64, _ := kelvin.Float64()
	return Kelvin(f64)
}

// String implements fmt.Stringer.
func (k Kelvin) String() string { return fmt.Sprintf("%gK", k) }

// Quantity implements qty.Measures[physical.Temperature].
func (k Kelvin) Quantity() (physical.Temperature, *big.Float, string) {
	return physical.Temperature{Kelvin: big.NewFloat(1)}, big.NewFloat(float64(k)), "K"
}

// Moles counts an amount of substance in moles.
type Moles float64

// MolesFrom converts a quantity of amount of substance to moles.
func MolesFrom(substance qty.Measures[physical.Substance]) Moles {
	unit, factor, _ := substance.Quantity()
	moles := unit.Moles.Mul(unit.Moles, factor)
	f64, _ := moles.Float64()
	return Moles(f64)
}

// String implements fmt.Stringer.
func (m Moles) String() string { return fmt.Sprintf("%gmol", m) }

// Quantity implements qty.Measures[physical.Substance].
func (m Moles) Quantity() (physical.Substance, *big.Float, string) {
	return physical.Substance{Moles: big.NewFloat(1)}, big.NewFloat(float64(m)), "mol"
}

// Candelas counts a luminous intensity in candelas.
type Candelas float64

// CandelasFrom converts a quantity of luminous intensity to candelas.
func CandelasFrom(brightness qty.Measures[physical.Brightness]) Candelas {
	unit, factor, _ := brightness.Quantity()
	candelas := unit.Candelas.Mul(unit.Candelas, factor)
	f64, _ := candelas.Float64()
	return Candelas(f64)
}

// String implements fmt.Stringer.
func (c Candelas) String() string { return fmt.Sprintf("%gcd", c) }

// Quantity implements qty.Measures[physical.Brightness].
func (c Candelas) Quantity() (physical.Brightness, *big.Float, string) {
	return physical.Brightness{Candelas: big.NewFloat(1)}, big.NewFloat(float64(c)), "cd"
}

// Bits counts a quantity of bits.
type Bits uint64

// BitsFrom converts a quantity of bits to bits.
func BitsFrom(information qty.Measures[physical.Information]) Bits {
	unit, factor, _ := information.Quantity()
	bits := unit.Bits.Mul(unit.Bits, factor)
	u64, _ := bits.Uint64()
	return Bits(u64)
}

// String implements fmt.Stringer.
func (b Bits) String() string { return fmt.Sprintf("%db", b) }

// Quantity implements qty.Measures[physical.Information].
func (b Bits) Quantity() (physical.Information, *big.Float, string) {
	var f big.Float
	f.SetUint64(uint64(b))
	return physical.Information{Bits: big.NewFloat(1)}, &f, "b"
}

// Bytes counts a quantity of bytes.
type Bytes uint64

// BytesFrom converts a quantity of bytes to bytes.
func BytesFrom(information qty.Measures[physical.Information]) Bytes {
	unit, factor, _ := information.Quantity()
	bits := unit.Bits.Mul(unit.Bits, factor)
	bits = bits.Quo(bits, big.NewFloat(8))
	u64, _ := bits.Uint64()
	return Bytes(u64)
}

// String implements fmt.Stringer.
func (b Bytes) String() string { return fmt.Sprintf("%dB", b) }

// Quantity implements qty.Measures[physical.Information].
func (b Bytes) Quantity() (physical.Information, *big.Float, string) {
	var f big.Float
	f.SetUint64(uint64(b))
	return physical.Information{Bits: big.NewFloat(8)}, &f, "B"
}
