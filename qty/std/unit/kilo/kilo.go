// Package kilo provides a standard way to represent thousands for a SI unit.
package kilo

import "time"

// Time counts a number of kiloseconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in kiloseconds.
type Seconds float64

// Metres counts a distance in kilometres.
type (
	Metres float64
	Meters = Metres
)

// Grams counts a mass in kilograms.
type Grams float64

// Amps counts a current in kiloamps.
type Amps float64

// Kelvin counts a temperature in kilokelvin.
type Kelvin float64

// Moles counts an amount of substance in kilomoles.
type Moles float64

// Candelas counts a luminous intensity in kilocandelas.
type Candelas float64

// Bytes counts a digital storage in kilobytes.
type Bytes float64
