// Package deci provides a standard way to represent a tenth of a SI unit.
package deci

import "time"

// Time counts a number of deciseconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in deciseconds.
type Seconds float64

// Metres counts a distance in decimetres.
type (
	Metres float64
	Meters = Metres
)

// Grams counts a mass in decigrams.
type Grams float64

// Amps counts a current in deciamps.
type Amps float64

// Kelvin counts a temperature in decikelvin.
type Kelvin float64

// Moles counts an amount of substance in decimoles.
type Moles float64

// Candelas counts a luminous intensity in decicandelas.
type Candelas float64
