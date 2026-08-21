package stduuid

import (
	"testing"
)

const want = "f81d4fae-7dec-11d0-a765-00a0c91e6bf6"

func TestParseForms(t *testing.T) {
	forms := []string{
		want,
		"f81d4fae7dec11d0a76500a0c91e6bf6",
		"{f81d4fae-7dec-11d0-a765-00a0c91e6bf6}",
		"urn:uuid:f81d4fae-7dec-11d0-a765-00a0c91e6bf6",
		"F81D4FAE-7DEC-11D0-A765-00A0C91E6BF6",
	}
	for _, s := range forms {
		u, err := Parse(s)
		if err != nil {
			t.Fatalf("Parse(%q): %v", s, err)
		}
		if got := u.String(); got != want {
			t.Errorf("Parse(%q).String() = %q, want %q", s, got, want)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	bad := []string{
		"",
		"not-a-uuid",
		"f81d4ffae--7dec-11d0-a765-00a0c91e6bf", // short
		"f81d4fae--7dec-11d0-a765-00a0c91e6bf6", // bad hex char

	}
	for _, s := range bad {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) succeeded", s)
		}
	}
}

func TestNilMax(t *testing.T) {
	if Nil().String() != "00000000-0000-0000-0000-000000000000" {
		t.Error("Nil mismatch")
	}
	if Max().String() != "ffffffff-ffff-ffff-ffff-ffffffffffff" {
		t.Error("Max mismatch")
	}
	if Nil() != (UUID{}) {
		t.Error("Nil is not the zero UUID")
	}
}

func TestNewV4(t *testing.T) {
	for i := 0; i < 100; i++ {
		u := NewV4()
		if u[6]>>4 != 4 {
			t.Fatalf("NewV4 version = %d", u[6]>>4)
		}
		if u[8]>>6 != 0b10 {
			t.Fatalf("NewV4 variant = %d", u[8]>>6)
		}
		if u == (UUID{}) {
			t.Fatal("NewV4 returned zero")
		}
	}
}

func TestNewV7(t *testing.T) {
	prev := ""
	for i := 0; i < 1000; i++ {
		u := NewV7()
		if u[6]>>4 != 7 {
			t.Fatalf("NewV7 version = %d", u[6]>>4)
		}
		if u[8]>>6 != 0b10 {
			t.Fatalf("NewV7 variant = %d", u[8]>>6)
		}
		s := u.String()
		if s <= prev {
			t.Fatalf("NewV7 not increasing: %q <= %q", s, prev)
		}
		prev = s
	}
}

func TestNew(t *testing.T) {
	if u := New(); u[6]>>4 != 4 {
		t.Fatalf("New version = %d, want 4", u[6]>>4)
	}
}

func TestMarshalTextRoundTrip(t *testing.T) {
	u := NewV4()
	b, err := u.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	var v UUID
	if err := v.UnmarshalText(b); err != nil {
		t.Fatal(err)
	}
	if v != u {
		t.Error("round trip mismatch")
	}
}

func TestCompare(t *testing.T) {
	a := MustParse("00000000-0000-0000-0000-000000000000")
	b := MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	if a.Compare(b) != -1 {
		t.Error("Compare ordering wrong")
	}
	if a.Compare(a) != 0 {
		t.Error("Compare equal wrong")
	}
	if b.Compare(a) != 1 {
		t.Error("Compare reverse wrong")
	}
}

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

func TestMustParsePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustParse did not panic on invalid input")
		}
	}()
	MustParse("not-a-uuid")
}

func TestUnmarshalTextErrors(t *testing.T) {
	bad := []string{
		"0000000000000000000000000000000x",      // 32-char compact, bad hex char
		"f81d4faeX7dec-11d0-a765-00a0c91e6bf6",  // 36-char, wrong dash position
		"f81d4fae-7-dec-11d0-a765-00a0c91e6bf6", // dash positions wrong
		"f81d4fae-7dec-11d0-a765-00a0c91e6bf6g", // bad hex in last group
	}
	for _, s := range bad {
		var u UUID
		if err := u.UnmarshalText([]byte(s)); err == nil {
			t.Errorf("UnmarshalText(%q) succeeded", s)
		}
	}
}

func TestUnmarshalHexDecodeErrors(t *testing.T) {
	// Each of these has correct dash positions (reaching the hex decode step)
	// but a nondigit character in one of the five hexadecimal groups.
	bad := []string{
		"x81d4fae-7dec-11d0-a765-00a0c91e6bf6", // group 1
		"f81d4fae-xdec-11d0-a765-00a0c91e6bf6", // group 2
		"f81d4fae-7dec-x1d0-a765-00a0c91e6bf6", // group 3
		"f81d4fae-7dec-11d0-x765-00a0c91e6bf6", // group 4
		"f81d4fae-7dec-11d0-a765-00a0c91e6bfx", // group 5
	}
	for i, s := range bad {
		var u UUID
		if err := u.UnmarshalText([]byte(s)); err == nil {
			t.Errorf("case %d UnmarshalText(%q) succeeded", i, s)
		}
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
