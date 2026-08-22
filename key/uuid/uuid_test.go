package uuid

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// canonical vectors for each RFC 9562 version. The version nibble is the high
// nibble of byte 6, i.e. the first hex character of the third group.
const (
	vecV1 = "f81d4fae-7dec-11d0-a765-00a0c91e6bf6"
	vecV2 = "00000000-0000-2000-8000-000000000000"
	vecV3 = "00000000-0000-3000-8000-000000000000"
	vecV4 = "f47ac10b-58cc-4372-a567-0e02b2c3d479"
	vecV5 = "00000000-0000-5000-8000-000000000000"
	vecV6 = "00000000-0000-6000-8000-000000000000"
	vecV7 = "00000000-0000-7000-8000-000000000000"
	vecV8 = "00000000-0000-8000-8000-000000000000"
)

func TestParseCanonicalForms(t *testing.T) {
	// The standard library parser accepts four textual forms; the public
	// package must canonicalize all of them to lowercase dashed text.
	inputs := []string{
		vecV1,
		"f81d4fae7dec11d0a76500a0c91e6bf6",              // compact
		"{f81d4fae-7dec-11d0-a765-00a0c91e6bf6}",        // braced
		"urn:uuid:f81d4fae-7dec-11d0-a765-00a0c91e6bf6", // urn
		"F81D4FAE-7DEC-11D0-A765-00A0C91E6BF6",          // uppercase
	}
	for _, in := range inputs {
		id, err := ParseV1[struct{}](in)
		if err != nil {
			t.Fatalf("ParseV1(%q): %v", in, err)
		}
		if got := string(id); got != vecV1 {
			t.Errorf("ParseV1(%q) = %q, want canonical %q", in, got, vecV1)
		}
	}
}

func TestParseVersions(t *testing.T) {
	cases := []struct {
		version byte
		vec     string
	}{
		{1, vecV1}, {2, vecV2}, {3, vecV3}, {4, vecV4},
		{5, vecV5}, {6, vecV6}, {7, vecV7}, {8, vecV8},
	}
	for _, c := range cases {
		got, err := parseAny(c.vec)
		if err != nil {
			t.Fatalf("version %d: %v", c.version, err)
		}
		if got != c.version {
			t.Errorf("version %d: parsed version = %d", c.version, got)
		}
	}
}

func TestParseWrongVersion(t *testing.T) {
	// A v4 string must not parse as v1.
	if _, err := ParseV1[struct{}](vecV4); err == nil {
		t.Error("ParseV1 accepted a v4 UUID")
	}
	if _, err := ParseV4[struct{}](vecV1); err == nil {
		t.Error("ParseV4 accepted a v1 UUID")
	}
}

func TestParseInvalid(t *testing.T) {
	bad := []string{
		"",
		"not-a-uuid",
		"f81d4fae-7dec-11d0-a765-00a0c91e6bf",   // too short
		"f81d4fae-7dec-11d0-a765-00a0c91e6bf6g", // bad hex char
		"f81d4fae7dec11d0a76500a0c91e6bf6g",     // bad hex compact
		"f81d4fae-7dec-11d0-a765-00a0c91e6bf6-", // trailing dash
		"00000000-0000-0000-0000-000000000000",  // nil (version 0)
		"00000000-0000-9000-8000-000000000000",  // version 9 (invalid)
		"00000000-0000-4000-0000-000000000000",  // non-RFC variant (00)
		"00000000-0000-4000-c000-000000000000",  // non-RFC variant (11)
		"urn:uuid:00000000-0000-4000-8000-000000000000x",
	}
	for _, s := range bad {
		if _, err := parseAny(s); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}

func TestValid(t *testing.T) {
	if !Valid(vecV4) {
		t.Error("Valid(v4) = false")
	}
	if !Valid(vecV1) {
		t.Error("Valid(v1) = false")
	}
	if Valid("") {
		t.Error("Valid(\"\") = true")
	}
	if Valid("00000000-0000-9000-8000-000000000000") {
		t.Error("Valid(version 9) = true")
	}
	if ValidV4(vecV1) {
		t.Error("ValidV4(v1) = true")
	}
	if !ValidV4(vecV4) {
		t.Error("ValidV4(v4) = false")
	}
}

func TestNew(t *testing.T) {
	id := New[struct{}]()
	if !ValidV4(string(id)) {
		t.Errorf("New() = %q, not a valid v4", id)
	}
	if string(id) == "" {
		t.Error("New() returned zero value")
	}
}

func TestNewV4(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := NewV4[struct{}]()
		if !ValidV4(string(id)) {
			t.Fatalf("NewV4() = %q, not a valid v4", id)
		}
		if seen[string(id)] {
			t.Fatalf("duplicate v4: %q", id)
		}
		seen[string(id)] = true
	}
}

func TestNewV7(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := NewV7[struct{}]()
		if !ValidV7(string(id)) {
			t.Fatalf("NewV7() = %q, not a valid v7", id)
		}
		if seen[string(id)] {
			t.Fatalf("duplicate v7: %q", id)
		}
		seen[string(id)] = true
	}
}

func TestNewV7Monotonic(t *testing.T) {
	const g = 8
	const per = 2000
	results := make([][]V7[struct{}], g)
	var wg sync.WaitGroup
	for i := 0; i < g; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = make([]V7[struct{}], per)
			for j := 0; j < per; j++ {
				results[i][j] = NewV7[struct{}]()
			}
		}(i)
	}
	wg.Wait()
	// v7 ids must be strictly increasing in string (lexicographic) order,
	// which matches the RFC 9562 big-endian byte ordering. Because generation
	// is globally monotonic (the underlying standard-library implementation
	// serializes same-timestamp calls), each goroutine's own sequence is
	// strictly increasing even though goroutines interleave.
	for i := 0; i < g; i++ {
		for j := 1; j < per; j++ {
			if string(results[i][j-1]) >= string(results[i][j]) {
				t.Fatalf("v7 ordering violated in goroutine %d at %d: %q >= %q",
					i, j, results[i][j-1], results[i][j])
			}
		}
	}
}

// boundaryID is the full operational contract shared by every identifier
// type, used to constrain the generic boundary test.
type boundaryID interface {
	~string
	fmt.Stringer
	encoding.TextMarshaler
	encoding.TextUnmarshaler
	json.Marshaler
	json.Unmarshaler
	sql.Scanner
	driver.Valuer
	Valid() bool
}

// boundaryValue is the value-receiver part of the identifier contract.
type boundaryValue interface {
	~string
	fmt.Stringer
	encoding.TextMarshaler
	json.Marshaler
	driver.Valuer
	Valid() bool
}

// boundaryPointer is the pointer-receiver part of the identifier contract.
type boundaryPointer[T boundaryValue] interface {
	*T
	encoding.TextUnmarshaler
	json.Unmarshaler
	sql.Scanner
}

// boundaryTest exercises the full operational contract for a string-backed
// identifier type of the given version.
func boundaryTest[T boundaryValue, P boundaryPointer[T]](t *testing.T, version byte, parse func(string) (T, error), cast func(string) T) {
	vec := map[byte]string{1: vecV1, 2: vecV2, 3: vecV3, 4: vecV4, 5: vecV5, 6: vecV6, 7: vecV7, 8: vecV8}[version]
	other := map[byte]string{1: vecV4, 2: vecV1, 3: vecV1, 4: vecV1, 5: vecV1, 6: vecV1, 7: vecV1, 8: vecV1}[version]

	id := cast(vec)
	zero := cast("")

	t.Run("String", func(t *testing.T) {
		if id.String() != vec {
			t.Errorf("String = %q, want %q", id.String(), vec)
		}
	})

	t.Run("Valid", func(t *testing.T) {
		if !id.Valid() {
			t.Error("Valid() = false for valid id")
		}
		if zero.Valid() {
			t.Error("Valid() = true for zero")
		}
		if cast(other).Valid() {
			t.Error("Valid() = true for wrong-version id")
		}
	})

	t.Run("ParseRoundTrip", func(t *testing.T) {
		got, err := parse(vec)
		if err != nil || string(got) != vec {
			t.Errorf("parse(%q) = %q, %v", vec, got, err)
		}
		if _, err := parse(""); err == nil {
			t.Error("parse(\"\") succeeded")
		}
	})

	t.Run("MarshalText", func(t *testing.T) {
		b, err := id.MarshalText()
		if err != nil || string(b) != vec {
			t.Errorf("MarshalText = %q, %v", b, err)
		}
		if _, err := zero.MarshalText(); err == nil {
			t.Error("MarshalText of zero succeeded")
		}
	})

	t.Run("UnmarshalText", func(t *testing.T) {
		var dst T = cast("sentinel")
		var p P = &dst
		if err := p.UnmarshalText([]byte(vec)); err != nil || string(dst) != vec {
			t.Errorf("UnmarshalText = %q, %v", dst, err)
		}
		// atomic on error
		dst = cast("sentinel")
		if err := p.UnmarshalText([]byte("bad")); err == nil {
			t.Error("UnmarshalText(bad) succeeded")
		} else if string(dst) != "sentinel" {
			t.Errorf("receiver mutated on error: %q", dst)
		}
	})

	t.Run("MarshalJSON", func(t *testing.T) {
		b, err := id.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var back string
		if err := json.Unmarshal(b, &back); err != nil || back != vec {
			t.Errorf("JSON round trip = %q, %v", back, err)
		}
		if _, err := zero.MarshalJSON(); err == nil {
			t.Error("MarshalJSON of zero succeeded")
		}
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		var dst T = cast("sentinel")
		var p P = &dst
		if err := p.UnmarshalJSON([]byte(`"` + vec + `"`)); err != nil || string(dst) != vec {
			t.Errorf("UnmarshalJSON = %q, %v", dst, err)
		}
		// null clears to zero
		if err := p.UnmarshalJSON([]byte("null")); err != nil || string(dst) != "" {
			t.Errorf("UnmarshalJSON(null) = %q, %v", dst, err)
		}
		// empty string is invalid
		dst = cast("sentinel")
		if err := p.UnmarshalJSON([]byte(`""`)); err == nil {
			t.Error("UnmarshalJSON(\"\") succeeded")
		} else if string(dst) != "sentinel" {
			t.Errorf("receiver mutated on error: %q", dst)
		}
		// non-string JSON is invalid
		for _, j := range []string{`123`, `{}`, `[]`, `true`} {
			dst = cast("sentinel")
			if err := p.UnmarshalJSON([]byte(j)); err == nil {
				t.Errorf("UnmarshalJSON(%s) succeeded", j)
			}
		}
	})

	t.Run("Scan", func(t *testing.T) {
		var dst T = cast("sentinel")
		var p P = &dst
		if err := p.Scan(vec); err != nil || string(dst) != vec {
			t.Errorf("Scan(string) = %q, %v", dst, err)
		}
		if err := p.Scan([]byte(vec)); err != nil || string(dst) != vec {
			t.Errorf("Scan([]byte) = %q, %v", dst, err)
		}
		if err := p.Scan(nil); err != nil || string(dst) != "" {
			t.Errorf("Scan(nil) = %q, %v", dst, err)
		}
		if err := p.Scan(42); err == nil {
			t.Error("Scan(int) succeeded")
		}
		dst = cast("sentinel")
		if err := p.Scan("bad"); err == nil {
			t.Error("Scan(bad) succeeded")
		} else if string(dst) != "sentinel" {
			t.Errorf("receiver mutated on error: %q", dst)
		}
	})

	t.Run("Value", func(t *testing.T) {
		v, err := id.Value()
		if err != nil || v != vec {
			t.Errorf("Value = %v, %v", v, err)
		}
		v, err = zero.Value()
		if err != nil || v != nil {
			t.Errorf("Value(zero) = %v, %v, want nil,nil", v, err)
		}
		if _, err := cast("bad").Value(); err == nil {
			t.Error("Value(invalid) succeeded")
		}
	})
}

func TestBoundaryAllVersions(t *testing.T) {
	boundaryTest(t, 1, func(s string) (V1[struct{}], error) { return ParseV1[struct{}](s) }, func(s string) V1[struct{}] { return V1[struct{}](s) })
	boundaryTest(t, 2, func(s string) (V2[struct{}], error) { return ParseV2[struct{}](s) }, func(s string) V2[struct{}] { return V2[struct{}](s) })
	boundaryTest(t, 3, func(s string) (V3[struct{}], error) { return ParseV3[struct{}](s) }, func(s string) V3[struct{}] { return V3[struct{}](s) })
	boundaryTest(t, 4, func(s string) (V4[struct{}], error) { return ParseV4[struct{}](s) }, func(s string) V4[struct{}] { return V4[struct{}](s) })
	boundaryTest(t, 5, func(s string) (V5[struct{}], error) { return ParseV5[struct{}](s) }, func(s string) V5[struct{}] { return V5[struct{}](s) })
	boundaryTest(t, 6, func(s string) (V6[struct{}], error) { return ParseV6[struct{}](s) }, func(s string) V6[struct{}] { return V6[struct{}](s) })
	boundaryTest(t, 7, func(s string) (V7[struct{}], error) { return ParseV7[struct{}](s) }, func(s string) V7[struct{}] { return V7[struct{}](s) })
	boundaryTest(t, 8, func(s string) (V8[struct{}], error) { return ParseV8[struct{}](s) }, func(s string) V8[struct{}] { return V8[struct{}](s) })
}

func TestInterfaceAssertions(t *testing.T) {
	var (
		_ fmt.Stringer             = V4[struct{}]("")
		_ encoding.TextMarshaler   = V4[struct{}]("")
		_ encoding.TextUnmarshaler = (*V4[struct{}])(nil)
		_ json.Marshaler           = V4[struct{}]("")
		_ json.Unmarshaler         = (*V4[struct{}])(nil)
		_ sql.Scanner              = (*V4[struct{}])(nil)
		_ driver.Valuer            = V4[struct{}]("")
	)
	_ = errors.Is
	_ = strings.TrimSpace
}

// parseAny returns the RFC version nibble of a well-formed UUID string.
func parseAny(s string) (byte, error) {
	id, err := parse(s)
	if err != nil {
		return 0, err
	}
	return versionOf(id), nil
}
