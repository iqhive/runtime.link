package nanoid

import (
	"errors"
	"testing"
)

// The tests in this file pin behaviour that callers' stored ids and wire
// values rely on, so it must not change.

func TestParseErrorsAreTheSentinels(t *testing.T) {
	cases := map[string]error{
		"bad":                   ErrSize,
		"V1StGXR8_Z5jdHi6B-my!": ErrCharacters,
	}
	for in, want := range cases {
		if _, err := Parse[struct{}](in); err != want {
			t.Errorf("Parse(%q) = %v, want %v itself", in, err, want)
		}
		var id V1[struct{}]
		if err := id.UnmarshalText([]byte(in)); err != want {
			t.Errorf("UnmarshalText(%q) = %v, want %v itself", in, err, want)
		}
		if err := id.UnmarshalJSON([]byte(`"` + in + `"`)); err != want {
			t.Errorf("UnmarshalJSON(%q) = %v, want %v itself", in, err, want)
		}
		if err := id.Scan(in); err != want {
			t.Errorf("Scan(%q) = %v, want %v itself", in, err, want)
		}
		if _, err := V1[struct{}](in).Value(); err != want {
			t.Errorf("Value(%q) = %v, want %v itself", in, err, want)
		}
	}
}

func TestEntropyErrorKeepsItsMessage(t *testing.T) {
	const want = "nanoid: nanoid: entropy source failed"
	_, err := NewWith[struct{}](errReader{})
	if !errors.Is(err, ErrEntropy) || err == ErrEntropy || err.Error() != want {
		t.Errorf("NewWith = %v, want %q", err, want)
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
