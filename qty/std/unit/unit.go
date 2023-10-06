// Package unit provides a standard way to represent a SI unit.
package unit

import "time"

// Time counts a number of seconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in seconds.
type Seconds float64

// Metres counts a distance in metres.
type (
	Metres float64
	Meters = Metres
)

// Grams counts a mass in grams.
type Grams float64

// Amps counts a current in amps.
type Amps float64

// Kelvin counts a temperature in kelvin.
type Kelvin float64

// Moles counts an amount of substance in moles.
type Moles float64

// Candelas counts a luminous intensity in candelas.
type Candelas float64
