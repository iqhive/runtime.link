//go:build !go1.27

// These tests poke at the internal monotonic-generator state of the bundled
// fallback implementation (uuid_legacy.go). They are tagged !go1.27 because
// those internals do not exist in the standard library uuid package used on
// Go 1.27+.
package stduuid

import "testing"

func TestNewV7ClockRollback(t *testing.T) {
	// Force the "clock went backwards" branch by setting the recorded last
	// second to the far future, then generate and confirm the state resets.
	v7mu.Lock()
	v7lastSecs = ^uint64(0)
	v7lastTimestamp = ^uint64(0)
	v7mu.Unlock()

	u := NewV7()
	if u[6]>>4 != 7 {
		t.Fatalf("NewV7 version = %d", u[6]>>4)
	}

	v7mu.Lock()
	defer v7mu.Unlock()
	if v7lastSecs == ^uint64(0) {
		t.Error("clock rollback did not reset v7lastSecs")
	}
}

func TestNewV7MonotonicExtension(t *testing.T) {
	// Force the "same (or earlier) timestamp" branch by recording a future
	// timestamp, which makes the current call take the extension path.
	v7mu.Lock()
	v7lastSecs = 0
	v7lastTimestamp = ^uint64(0)
	v7mu.Unlock()

	u := NewV7()
	if u[6]>>4 != 7 {
		t.Fatalf("NewV7 version = %d", u[6]>>4)
	}

	v7mu.Lock()
	defer v7mu.Unlock()
	if v7lastTimestamp == ^uint64(0) {
		t.Error("monotonic extension did not advance the timestamp")
	}
}
