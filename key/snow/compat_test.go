package snow

import (
	"encoding/json"
	"testing"
)

// The tests in this file pin behaviour that callers' stored ids and wire
// values rely on, so it must not change.

func TestNewUsesWorkerZero(t *testing.T) {
	for range 100 {
		id := New[struct{}]()
		if worker := (int64(id) >> sequenceBits) & (1<<workerBits - 1); worker != 0 {
			t.Fatalf("New() = %d with worker %d, want worker 0", int64(id), worker)
		}
	}
}

func TestParseAcceptsSignsLeadingZerosAndZero(t *testing.T) {
	cases := map[string]int64{"+5": 5, "-0": 0, "+0": 0, "0005": 5, "0": 0, "00": 0}
	for s, want := range cases {
		if id, err := Parse[struct{}](s); err != nil || int64(id) != want {
			t.Errorf("Parse(%q) = %d, %v, want %d", s, int64(id), err, want)
		}
		if !Valid(s) {
			t.Errorf("Valid(%q) = false", s)
		}
		id := FlakeID[struct{}](7)
		if err := id.UnmarshalText([]byte(s)); err != nil || int64(id) != want {
			t.Errorf("UnmarshalText(%q) = %d, %v, want %d", s, int64(id), err, want)
		}
		for _, src := range []any{s, []byte(s)} {
			id := FlakeID[struct{}](7)
			if err := id.Scan(src); err != nil || int64(id) != want {
				t.Errorf("Scan(%#v) = %d, %v, want %d", src, int64(id), err, want)
			}
		}
	}
}

// Negatives are refused, see TestRejectNegativeIDs.
func TestUnmarshalJSONKeepsZero(t *testing.T) {
	for _, j := range []string{`-0`, `0`} {
		id := FlakeID[struct{}](7)
		if err := id.UnmarshalJSON([]byte(j)); err != nil || id != 0 {
			t.Errorf("UnmarshalJSON(%s) = %d, %v, want 0", j, int64(id), err)
		}
	}
}

func TestUnmarshalJSONErrorsKeepTheirMessages(t *testing.T) {
	for _, j := range []string{`1.5`, `1e3`, `99999999999999999999`, `"abc"`, `"+5"`, `"0"`, `"-5"`, `""`, `true`, `{}`, `[]`} {
		var n int64
		want := "snow: " + json.Unmarshal([]byte(j), &n).Error()
		id := FlakeID[struct{}](7)
		err := id.UnmarshalJSON([]byte(j))
		if err == nil || err.Error() != want || id != 7 {
			t.Errorf("UnmarshalJSON(%s) = %d, %v, want 7, %q", j, int64(id), err, want)
		}
	}
}
