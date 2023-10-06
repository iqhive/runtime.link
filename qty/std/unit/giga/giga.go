// Package giga provides a standard way to represent billions for a SI unit.
package giga

import "time"

// Time counts a number of gigaseconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in gigaseconds.
type Seconds float64

// Metres counts a distance in gigametres.
type (
	Metres float64
	Meters = Metres
)

// Grams counts a mass in gigagrams.
type Grams float64

// Amps counts a current in gigaamps.
type Amps float64

// Kelvin counts a temperature in gigakelvin.
type Kelvin float64

// Moles counts an amount of substance in gigamoles.
type Moles float64

// Candelas counts a luminous intensity in gigacandelas.
type Candelas float64

// Bytes counts a digital storage in gigabytes.
type Bytes float64
