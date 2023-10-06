// Package micro provides a standard way to represent a hundred thousandth of a SI unit.
package micro

import "time"

// Time counts a number of microseconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in microseconds.
type Seconds float64

// Metres counts a distance in micrometres.
type (
	Metres float64
	Meters = Metres
)

// Grams counts a mass in micrograms.
type Grams float64

// Amps counts a current in microamps.
type Amps float64

// Kelvin counts a temperature in microkelvin.
type Kelvin float64

// Moles counts an amount of substance in micromoles.
type Moles float64

// Candelas counts a luminous intensity in microcandelas.
type Candelas float64
