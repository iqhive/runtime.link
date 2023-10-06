// Package milli provides a standard way to represent a thousandth of a SI unit.
package milli

import "time"

// Time counts a number of milliseconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in milliseconds.
type Seconds float64

// Metres counts a distance in millimetres.
type (
	Metres float64
	Meters = Metres
)

// Grams counts a mass in milligrams.
type Grams float64

// Amps counts a current in milliamps.
type Amps float64

// Kelvin counts a temperature in millikelvin.
type Kelvin float64

// Moles counts an amount of substance in millimoles.
type Moles float64

// Candelas counts a luminous intensity in millicandelas.
type Candelas float64
