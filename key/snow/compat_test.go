package snow

import (
	"encoding/json"
	"math"
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

func TestUnmarshalJSONKeepsNegativesAndZero(t *testing.T) {
	cases := map[string]int64{`-5`: -5, `-0`: 0, `0`: 0, `-9223372036854775808`: math.MinInt64}
	for j, want := range cases {
		id := FlakeID[struct{}](7)
		if err := id.UnmarshalJSON([]byte(j)); err != nil || int64(id) != want {
			t.Errorf("UnmarshalJSON(%s) = %d, %v, want %d", j, int64(id), err, want)
		}
	}
	var out struct{ ID FlakeID[struct{}] }
	if err := json.Unmarshal([]byte(`{"ID":-5}`), &out); err != nil || out.ID != -5 {
		t.Errorf("json.Unmarshal = %d, %v", int64(out.ID), err)
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

func TestScanKeepsNegativeIntegers(t *testing.T) {
	id := FlakeID[struct{}](7)
	if err := id.Scan(int64(-5)); err != nil || id != -5 {
		t.Errorf("Scan(int64(-5)) = %d, %v", int64(id), err)
	}
}

func TestNegativeIDsAreValidAndEncode(t *testing.T) {
	negative := FlakeID[struct{}](-5)
	if !negative.Valid() {
		t.Error("Valid(-5) = false")
	}
	if b, err := negative.MarshalText(); err != nil || string(b) != "-5" {
		t.Errorf("MarshalText(-5) = %q, %v", b, err)
	}
	if b, err := negative.MarshalJSON(); err != nil || string(b) != "-5" {
		t.Errorf("MarshalJSON(-5) = %s, %v", b, err)
	}
	if v, err := negative.Value(); err != nil || v != int64(-5) {
		t.Errorf("Value(-5) = %v, %v", v, err)
	}
	b, err := json.Marshal(struct{ ID FlakeID[struct{}] }{-5})
	if err != nil || string(b) != `{"ID":-5}` {
		t.Errorf("json.Marshal = %s, %v", b, err)
	}
}
