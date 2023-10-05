// Package unix provides standard unix time types.
package unix

import "time"

// Epoch is the unix epoch.
type Epoch struct{}

// Time returns the unix epoch as a [time.Time].
func (Epoch) Time() time.Time { return time.Unix(0, 0) }
