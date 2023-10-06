// Package mega provides standard types to represent millions for a SI unit.
package mega

import "time"

// Time counts a number of megaseconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in megaseconds.
type Seconds float64

// Metres counts a distance in megametres.
type (
	Metres float64
	Meters = Metres
)

// Grams counts a mass in megagrams.
type Grams float64

// Amps counts a current in megaamps.
type Amps float64

// Kelvin counts a temperature in megakelvin.
type Kelvin float64

// Moles counts an amount of substance in megamoles.
type Moles float64

// Candelas counts a luminous intensity in megacandelas.
type Candelas float64

// Bytes counts a digital storage in megabytes.
type Bytes float64
