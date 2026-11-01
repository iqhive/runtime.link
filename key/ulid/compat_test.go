package ulid

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

// The tests in this file pin behaviour that callers' stored ids and wire
// values rely on, so it must not change.

// timestampSpan is the number of milliseconds the 48-bit timestamp holds.
const timestampSpan = 1 << 48

func TestNewWithWrapsTimesOutsideTheTimestamp(t *testing.T) {
	cases := map[string]struct{ at, same time.Time }{
		"1 ms before the epoch":  {time.UnixMilli(-1), time.UnixMilli(timestampSpan - 1)},
		"2^48 ms":                {time.UnixMilli(timestampSpan), time.UnixMilli(0)},
		"2^48 ms plus 5 seconds": {time.UnixMilli(timestampSpan + 5000), time.UnixMilli(5000)},
		"zero time":              {time.Time{}, time.UnixMilli(timestampSpan - 62135596800000)},
	}
	for name, c := range cases {
		got, err := NewWith[struct{}](c.at, bytes.NewReader(make([]byte, 10)))
		want, wantErr := NewWith[struct{}](c.same, bytes.NewReader(make([]byte, 10)))
		if err != nil || wantErr != nil || got != want {
			t.Errorf("%s: NewWith = %q, %v, want %q, %v", name, got, err, want, wantErr)
		}
	}
	got, err := NewWith[struct{}](time.UnixMilli(-1), bytes.NewReader(make([]byte, 10)))
	if err != nil || got != "7ZZZZZZZZZ0000000000000000" {
		t.Errorf("NewWith(1 ms before the epoch) = %q, %v", got, err)
	}
}

func TestNewWrapsAClockOutsideTheTimestamp(t *testing.T) {
	ent := fixedEntropy{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	cases := map[string]struct{ at, same time.Time }{
		"1 ms before the epoch": {time.UnixMilli(-1), time.UnixMilli(timestampSpan - 1)},
		"2^48 ms":               {time.UnixMilli(timestampSpan), time.UnixMilli(0)},
	}
	for name, c := range cases {
		useFreshGenerator(t, ent)
		got, err := generateMonotonic(c.at)
		want, wantErr := NewWith[struct{}](c.same, ent)
		if err != nil || wantErr != nil || got != string(want) {
			t.Errorf("%s: id = %q, %v, want %q, %v", name, got, err, want, wantErr)
		}
	}
}

func TestNewAtTheEpochOnAFreshGeneratorCountsFromZero(t *testing.T) {
	// A fresh generator's last millisecond is 0, so a clock at the epoch takes
	// the same-millisecond path and increments the zero entropy.
	useFreshGenerator(t, fixedEntropy{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	got, err := generateMonotonic(time.UnixMilli(0))
	if err != nil || got != "00000000000000000000000001" {
		t.Errorf("id = %q, %v", got, err)
	}
}

func TestParseReportsOverflowForAFirstCharacterAboveSeven(t *testing.T) {
	const rest = "1ARZ3NDEKTSV4RRFFQ69G5FAV"
	for _, first := range []string{"U", "I", "L", "O", "u", "~", "\xff", "8", "Z", "z"} {
		_, err := Parse[struct{}](first + rest)
		if !errors.Is(err, ErrOverflow) || err.Error() != "ulid: ulid: overflow" {
			t.Errorf("Parse(%q + rest) = %v, want ErrOverflow", first, err)
		}
	}
	for _, first := range []string{"!", "-"} {
		if _, err := Parse[struct{}](first + rest); !errors.Is(err, ErrCharacters) {
			t.Errorf("Parse(%q + rest) = %v, want ErrCharacters", first, err)
		}
	}
}

func TestParseErrorsUnwrapToTheirSentinel(t *testing.T) {
	cases := map[string]error{
		"bad":                        ErrSize,
		"01ARZ3NDEKTSV4RRFFQ69G5FA-": ErrCharacters,
		"8ZZZZZZZZZZZZZZZZZZZZZZZZZ": ErrOverflow,
	}
	for in, want := range cases {
		if _, err := Parse[struct{}](in); errors.Unwrap(err) != want {
			t.Errorf("errors.Unwrap(Parse(%q)) = %v, want %v", in, errors.Unwrap(err), want)
		}
	}
}

func TestEntropyErrorsKeepTheirMessage(t *testing.T) {
	const want = "ulid: ulid: entropy source failed"
	check := func(name string, err error) {
		t.Helper()
		if !errors.Is(err, ErrEntropy) || err == ErrEntropy || err.Error() != want {
			t.Errorf("%s = %v, want %q", name, err, want)
		}
	}
	_, err := NewWith[struct{}](time.Now(), errReader{})
	check("NewWith", err)

	useFreshGenerator(t, errReader{})
	_, err = generateMonotonic(time.Now())
	check("generateMonotonic", err)

	defer func() {
		err, _ := recover().(error)
		check("New panic", err)
	}()
	New[struct{}]()
}

func TestOutputIsTheTextHeld(t *testing.T) {
	for _, in := range []string{"01arz3ndektsv4rrffq69g5fav", "01ArZ3nDeKtSv4RrFfQ69G5fAv"} {
		id := V1[struct{}](in)
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
	bad := V1[struct{}]("bad")
	if b, err := bad.MarshalText(); err != nil || string(b) != "bad" {
		t.Errorf("MarshalText(bad) = %s, %v", b, err)
	}
	if b, err := bad.MarshalJSON(); err != nil || string(b) != `"bad"` {
		t.Errorf("MarshalJSON(bad) = %s, %v", b, err)
	}
}
