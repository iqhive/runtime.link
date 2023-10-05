// Package nano provides a standard way to represent a millionth for a SI unit.
package nano

import "time"

// Time counts a number of nanoseconds since the specified epoch.
type Time[Epoch interface{ Time() time.Time }] int64

// Seconds counts a duration of time in nanoseconds.
type Seconds = time.Duration
