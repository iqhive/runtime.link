package ksuid

import (
	"errors"
	"math"
	"testing"
	"time"
)

// The tests in this file pin behaviour that callers' stored ids and wire
// values rely on, so it must not change.

// wrappedStamp is the timestamp a time gets when its offset from the epoch is
// truncated to 32 bits.
func wrappedStamp(at time.Time) uint32 { return uint32(at.Unix() - epochStamp) }

func TestNewFromPartsWrapsTimesOutsideTheTimestamp(t *testing.T) {
	cases := map[string]time.Time{
		"zero time":               {},
		"2013":                    time.Date(2013, 1, 1, 0, 0, 0, 0, time.UTC),
		"1 ns before the epoch":   time.Unix(epochStamp, -1),
		"2200":                    time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC),
		"just past the last time": time.Unix(epochStamp+math.MaxUint32+1, 0),
	}
	for name, at := range cases {
		id, err := NewFromParts[struct{}](at, make([]byte, 16))
		if err != nil {
			t.Errorf("%s: NewFromParts = %q, %v", name, id, err)
			continue
		}
		if got, want := stampOf(t, string(id)), wrappedStamp(at); got != want {
			t.Errorf("%s: stamp %d, want %d", name, got, want)
		}
	}
}

func TestGenerateWrapsAClockOutsideTheTimestamp(t *testing.T) {
	for _, at := range []time.Time{time.Unix(0, 0), time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC)} {
		id, err := generate(at)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := stampOf(t, id), wrappedStamp(at); got != want {
			t.Errorf("generate(%v) stamp %d, want %d", at, got, want)
		}
	}
}

func TestEntropyErrorKeepsItsMessage(t *testing.T) {
	old := randEntropy
	randEntropy = errReader{}
	defer func() { randEntropy = old }()

	const want = "ksuid: ksuid: entropy source failed"
	_, err := generate(time.Now())
	if !errors.Is(err, ErrEntropy) || err == ErrEntropy || err.Error() != want {
		t.Errorf("generate = %v, want %q", err, want)
	}
}

func TestMarshalEmitsTheTextHeld(t *testing.T) {
	bad := V1[struct{}]("bad")
	if b, err := bad.MarshalText(); err != nil || string(b) != "bad" {
		t.Errorf("MarshalText(bad) = %s, %v", b, err)
	}
	if b, err := bad.MarshalJSON(); err != nil || string(b) != `"bad"` {
		t.Errorf("MarshalJSON(bad) = %s, %v", b, err)
	}
}
