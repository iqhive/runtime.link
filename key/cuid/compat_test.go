package cuid

import (
	"errors"
	"testing"
)

// The tests in this file pin behaviour that callers' stored ids and wire
// values rely on, so it must not change.

func TestSecureRandomPanicKeepsItsMessage(t *testing.T) {
	old := randReader
	randReader = errReader{}
	defer func() { randReader = old }()

	const want = "cuid: cuid: secure random source failed"
	defer func() {
		err, _ := recover().(error)
		if !errors.Is(err, ErrSecure) || err == ErrSecure || err.Error() != want {
			t.Errorf("recovered %v, want %q", err, want)
		}
	}()
	secureRandom()
}

func TestMarshalEmitsTheTextHeld(t *testing.T) {
	check := func(name string, text func() ([]byte, error), js func() ([]byte, error)) {
		t.Helper()
		if b, err := text(); err != nil || string(b) != "bad" {
			t.Errorf("%s MarshalText(bad) = %s, %v", name, b, err)
		}
		if b, err := js(); err != nil || string(b) != `"bad"` {
			t.Errorf("%s MarshalJSON(bad) = %s, %v", name, b, err)
		}
	}
	v1, v2 := V1[struct{}]("bad"), V2[struct{}]("bad")
	check("V1", v1.MarshalText, v1.MarshalJSON)
	check("V2", v2.MarshalText, v2.MarshalJSON)
}
