// Package hecto provides a standard way to represent hundreds for a SI unit.
package hecto

import "time"

// Time counts a number of hectoseconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in hectoseconds.
type Seconds float64

// Metres counts a distance in hectometres.
type (
	Metres float64
	Meters = Metres
)

// Grams counts a mass in hectograms.
type Grams float64

// Amps counts a current in hectoamps.
type Amps float64

// Kelvin counts a temperature in hectokelvin.
type Kelvin float64

// Moles counts an amount of substance in hectomoles.
type Moles float64

// Candelas counts a luminous intensity in hectocandelas.
type Candelas float64

// Bytes counts a digital storage in hectobytes.
type Bytes float64
