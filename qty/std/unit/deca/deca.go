// Package deca provides a standard way to represent tens for a SI unit.
package deca

import "time"

// Time counts a number of decaseconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in decaseconds.
type Seconds float64

// Metres counts a distance in decametres.
type (
	Metres float64
	Meters = Metres
)

// Grams counts a mass in decagrams.
type Grams float64

// Amps counts a current in decaamps.
type Amps float64

// Kelvin counts a temperature in decakelvin.
type Kelvin float64

// Moles counts an amount of substance in decamoles.
type Moles float64

// Candelas counts a luminous intensity in decacandelas.
type Candelas float64

// Bytes counts a digital storage in decabytes.
type Bytes float64
