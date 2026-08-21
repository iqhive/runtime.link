package snow

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	seen := map[int64]bool{}
	for i := 0; i < 5000; i++ {
		id := New[struct{}]()
		v := int64(id)
		if v <= 0 {
			t.Fatalf("New() = %d, must be positive", v)
		}
		if !Valid(strconv.FormatInt(v, 10)) {
			t.Fatalf("New() = %d, not valid", v)
		}
		if seen[v] {
			t.Fatalf("duplicate snowflake: %d", v)
		}
		seen[v] = true
	}
}

func TestNewMonotonic(t *testing.T) {
	const g = 8
	const per = 2000
	results := make([][]FlakeID[struct{}], g)
	var wg sync.WaitGroup
	for i := 0; i < g; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = make([]FlakeID[struct{}], per)
			for j := 0; j < per; j++ {
				results[i][j] = New[struct{}]()
			}
		}(i)
	}
	wg.Wait()
	for i := 0; i < g; i++ {
		for j := 1; j < per; j++ {
			if results[i][j-1] >= results[i][j] {
				t.Fatalf("snowflake not monotonic in goroutine %d at %d: %d >= %d",
					i, j, results[i][j-1], results[i][j])
			}
		}
	}
}

func TestParse(t *testing.T) {
	id := New[struct{}]()
	s := strconv.FormatInt(int64(id), 10)
	got, err := Parse[struct{}](s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	if got != id {
		t.Errorf("Parse = %d, want %d", got, id)
	}
}

func TestParseInvalid(t *testing.T) {
	bad := []string{
		"",
		"abc",
		"12.5",
		" 12",
		"12 ",
		"-5",
		"999999999999999999999999999999999999999999", // overflow int64
	}
	for _, s := range bad {
		if _, err := Parse[struct{}](s); err == nil {
			t.Errorf("Parse(%q) succeeded", s)
		}
	}
}

func TestValid(t *testing.T) {
	if !Valid("12345") {
		t.Error("Valid(\"12345\") = false")
	}
	if Valid("") {
		t.Error("Valid(\"\") = true")
	}
	if Valid("abc") {
		t.Error("Valid(\"abc\") = true")
	}
	if Valid("-1") {
		t.Error("Valid(\"-1\") = true")
	}
}

func TestGenerateLayout(t *testing.T) {
	// Snowflake generation is monotonic, so two calls at the same time yield
	// consecutive sequence values. Verify the bit layout: worker is bits
	// 12..21, sequence is bits 0..11.
	ts := time.Unix(1_700_000_000, 0)
	a := generate(ts, 3)
	b := generate(ts, 3)
	if a == b {
		t.Error("two calls at the same time produced the same id")
	}
	if (a>>12)&0x3FF != 3 {
		t.Errorf("worker bits wrong: %d", (a>>12)&0x3FF)
	}
	if b-a != 1 {
		t.Errorf("expected consecutive sequence, got %d -> %d", a, b)
	}
	// different workers produce different ids
	c := generate(time.Unix(1_700_000_000, 1), 4)
	if (c>>12)&0x3FF != 4 {
		t.Errorf("worker bits wrong for worker 4: %d", (c>>12)&0x3FF)
	}
}

func TestGenerateClockBackwards(t *testing.T) {
	// Force the clock-backwards branch by generating at a future time first.
	generate(time.Unix(1_800_000_000, 0), 1)
	// A call at an earlier time must still produce a monotonically increasing
	// id (clamped to the previous timestamp).
	earlier := generate(time.Unix(1_700_000_000, 0), 1)
	future := generate(time.Unix(1_800_000_000, 0), 1)
	if earlier >= future {
		t.Errorf("clock rollback produced non-increasing id: %d >= %d", earlier, future)
	}
}

func TestBoundary(t *testing.T) {
	id := New[struct{}]()
	zero := FlakeID[struct{}](0)
	text := strconv.FormatInt(int64(id), 10)

	if id.String() != text {
		t.Errorf("String = %q, want %q", id.String(), text)
	}
	if !id.Valid() {
		t.Error("Valid() = false")
	}
	if zero.Valid() {
		t.Error("Valid(zero) = true")
	}

	// MarshalText
	b, err := id.MarshalText()
	if err != nil || string(b) != text {
		t.Errorf("MarshalText = %q, %v", b, err)
	}
	if _, err := zero.MarshalText(); err == nil {
		t.Error("MarshalText(zero) succeeded")
	}

	// UnmarshalText
	var dst FlakeID[struct{}] = FlakeID[struct{}](999)
	if err := dst.UnmarshalText([]byte(text)); err != nil || dst != id {
		t.Errorf("UnmarshalText = %d, %v", dst, err)
	}
	dst = FlakeID[struct{}](999)
	if err := dst.UnmarshalText([]byte("bad")); err == nil {
		t.Error("UnmarshalText(bad) succeeded")
	} else if dst != FlakeID[struct{}](999) {
		t.Errorf("receiver mutated on error: %d", dst)
	}

	// MarshalJSON (numeric)
	jb, err := id.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(jb) != text {
		t.Errorf("MarshalJSON = %s, want %s", jb, text)
	}
	if _, err := zero.MarshalJSON(); err == nil {
		t.Error("MarshalJSON(zero) succeeded")
	}

	// UnmarshalJSON
	var jd FlakeID[struct{}]
	if err := jd.UnmarshalJSON([]byte(text)); err != nil || jd != id {
		t.Errorf("UnmarshalJSON = %d, %v", jd, err)
	}
	if err := jd.UnmarshalJSON([]byte("null")); err != nil || jd != 0 {
		t.Errorf("UnmarshalJSON(null) = %d, %v", jd, err)
	}
	for _, j := range []string{`"` + text + `"`, `{}`, `[]`, `true`, `1.5`} {
		if err := jd.UnmarshalJSON([]byte(j)); err == nil {
			t.Errorf("UnmarshalJSON(%s) succeeded", j)
		}
	}

	// Scan
	var sd FlakeID[struct{}]
	if err := sd.Scan(int64(id)); err != nil || sd != id {
		t.Errorf("Scan(int64) = %d, %v", sd, err)
	}
	if err := sd.Scan(text); err != nil || sd != id {
		t.Errorf("Scan(string) = %d, %v", sd, err)
	}
	if err := sd.Scan([]byte(text)); err != nil || sd != id {
		t.Errorf("Scan([]byte) = %d, %v", sd, err)
	}
	if err := sd.Scan(nil); err != nil || sd != 0 {
		t.Errorf("Scan(nil) = %d, %v", sd, err)
	}
	if err := sd.Scan("bad"); err == nil {
		t.Error("Scan(bad) succeeded")
	}
	if err := sd.Scan(1.5); err == nil {
		t.Error("Scan(float64) succeeded")
	}

	// Value
	v, err := id.Value()
	if err != nil || v != int64(id) {
		t.Errorf("Value = %v, %v", v, err)
	}
	v, err = zero.Value()
	if err != nil || v != nil {
		t.Errorf("Value(zero) = %v, %v", v, err)
	}
}

func TestInterfaceAssertions(t *testing.T) {
	var (
		_ fmt.Stringer             = FlakeID[struct{}](0)
		_ encoding.TextMarshaler   = FlakeID[struct{}](0)
		_ encoding.TextUnmarshaler = (*FlakeID[struct{}])(nil)
		_ json.Marshaler           = FlakeID[struct{}](0)
		_ json.Unmarshaler         = (*FlakeID[struct{}])(nil)
		_ sql.Scanner              = (*FlakeID[struct{}])(nil)
		_ driver.Valuer            = FlakeID[struct{}](0)
	)
	_ = errors.Is
}
