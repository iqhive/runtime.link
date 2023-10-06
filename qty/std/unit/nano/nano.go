// Package nano provides a standard way to represent a millionth of a SI unit.
package nano

import "time"

// Time counts a number of nanoseconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in nanoseconds.
type Seconds float64

// Metres counts a distance in nanometres.
type (
	Metres float64
	Meters = Metres
)

// Grams counts a mass in nanograms.
type Grams float64

// Amps counts a current in nanoamps.
type Amps float64

// Kelvin counts a temperature in nanokelvin.
type Kelvin float64

// Moles counts an amount of substance in nanomoles.
type Moles float64

// Candelas counts a luminous intensity in nanocandelas.
type Candelas float64
