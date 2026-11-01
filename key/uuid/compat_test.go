package uuid

import "testing"

// The tests in this file pin behaviour that callers' stored ids and wire
// values rely on, so it must not change.

func TestOutputIsTheTextHeld(t *testing.T) {
	for _, in := range []string{
		"F47AC10B-58CC-4372-A567-0E02B2C3D479",
		"{f47ac10b-58cc-4372-a567-0e02b2c3d479}",
		"urn:uuid:f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"f47ac10b58cc4372a5670e02b2c3d479",
	} {
		id := V4[struct{}](in)
		if v, err := id.Value(); err != nil || v != in {
			t.Errorf("Value(%q) = %v, %v", in, v, err)
		}
		if b, err := id.MarshalText(); err != nil || string(b) != in {
			t.Errorf("MarshalText(%q) = %s, %v", in, b, err)
		}
		if b, err := id.MarshalJSON(); err != nil || string(b) != `"`+in+`"` {
			t.Errorf("MarshalJSON(%q) = %s, %v", in, b, err)
		}
	}
	for _, bad := range []string{"bad", vecV1} {
		id := V4[struct{}](bad)
		if b, err := id.MarshalText(); err != nil || string(b) != bad {
			t.Errorf("MarshalText(%q) = %s, %v", bad, b, err)
		}
		if b, err := id.MarshalJSON(); err != nil || string(b) != `"`+bad+`"` {
			t.Errorf("MarshalJSON(%q) = %s, %v", bad, b, err)
		}
	}
}
