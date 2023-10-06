// Package centi provides standard types to represent a hundredth of a SI unit.
package centi

import "time"

// Time counts a number of centiseconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in centiseconds.
type Seconds float64

// Metres counts a distance in centimetres.
type (
	Metres float64
	Meters = Metres
)

// Grams counts a mass in centigrams.
type Grams float64

// Amps counts a current in centiamps.
type Amps float64

// Kelvin counts a temperature in centikelvin.
type Kelvin float64

// Moles counts an amount of substance in centimoles.
type Moles float64

// Candelas counts a luminous intensity in centicandelas.
type Candelas float64
