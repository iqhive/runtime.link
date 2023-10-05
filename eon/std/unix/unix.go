// Package unix provides standard unix time types.
package unix

import "time"

// Epoch is the unix epoch.
var Epoch = time.Unix(0, 0)

// Nanos is a timestamp in nanoseconds since the unix epoch.
type Nanos int64

// Micros is a timestamp in microseconds since the unix epoch.
type Micros int64

// Millis is a timestamp in milliseconds since the unix epoch.
type Millis int64
