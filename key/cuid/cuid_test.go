package cuid

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

const (
	vecV1 = "cjld2cjxh0000qzrmn831i7rn"
	vecV2 = "tz4a98xxat96iws9zmbrgj3a"
)

func TestParseV1(t *testing.T) {
	id, err := ParseV1[struct{}](vecV1)
	if err != nil {
		t.Fatalf("ParseV1(%q): %v", vecV1, err)
	}
	if string(id) != vecV1 {
		t.Errorf("ParseV1 = %q", id)
	}
}

func TestParseV1Invalid(t *testing.T) {
	bad := []string{
		"",
		"cjld2cjxh0000qzrmn831i7r",   // too short
		"cjld2cjxh0000qzrmn831i7rnX", // uppercase char
		"cjld2cjxh0000qzrmn831i7r-",  // invalid char
		"xjld2cjxh0000qzrmn831i7rn",  // does not start with c
		"cjld2cjxh0000qzrmn831i7rn ", // trailing space
	}
	for _, s := range bad {
		if _, err := ParseV1[struct{}](s); err == nil {
			t.Errorf("ParseV1(%q) succeeded", s)
		}
	}
}

func TestParseV2(t *testing.T) {
	id, err := ParseV2[struct{}](vecV2)
	if err != nil {
		t.Fatalf("ParseV2(%q): %v", vecV2, err)
	}
	if string(id) != vecV2 {
		t.Errorf("ParseV2 = %q", id)
	}
}

func TestParseV2Invalid(t *testing.T) {
	bad := []string{
		"",
		"tz4a98xxat96iws9zmbrgj3",   // too short
		"tz4a98xxat96iws9zmbrgj3aZ", // uppercase
		"Tz4a98xxat96iws9zmbrgj3a",  // first char uppercase
		"0z4a98xxat96iws9zmbrgj3a",  // first char not a letter
		"tz4a98xxat96iws9zmbrgj3a-", // invalid char
		"tz4a98xxat96iws9zmbrgj3-",  // invalid char, correct length
	}
	for _, s := range bad {
		if _, err := ParseV2[struct{}](s); err == nil {
			t.Errorf("ParseV2(%q) succeeded", s)
		}
	}
}

func TestValid(t *testing.T) {
	if !ValidV1(vecV1) {
		t.Error("ValidV1(v1) = false")
	}
	if !ValidV2(vecV2) {
		t.Error("ValidV2(v2) = false")
	}
	if ValidV1(vecV2) {
		t.Error("ValidV1(v2) = true")
	}
	if ValidV2(vecV1) {
		t.Error("ValidV2(v1) = true")
	}
}

func TestNew(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		id := New[struct{}]()
		s := string(id)
		if len(s) != 24 {
			t.Fatalf("New() length = %d, want 24", len(s))
		}
		if !ValidV2(s) {
			t.Fatalf("New() = %q, not a valid cuid2", s)
		}
		if seen[s] {
			t.Fatalf("duplicate cuid2: %q", s)
		}
		seen[s] = true
	}
}

// deterministicRand returns a deterministic [0,1) generator cycling through
// the provided values.
func deterministicRand(values []float64) func() float64 {
	i := 0
	return func() float64 {
		v := values[i%len(values)]
		i++
		return v
	}
}

func fixedNow() time.Time { return time.Unix(1_700_000_000, 0) }

func TestNewWithDeterministic(t *testing.T) {
	// Identical (random stream, time, fingerprint, counter seed) inputs must
	// produce the identical id, proving the algorithm has no hidden state.
	values := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9}

	a := generateCUID2(deterministicRand(values), fixedNow, "fingerprint-value", newCounterWithSeed(12345))
	b := generateCUID2(deterministicRand(values), fixedNow, "fingerprint-value", newCounterWithSeed(12345))
	if a != b {
		t.Errorf("deterministic generation produced different ids: %q vs %q", a, b)
	}
}

func TestHashStable(t *testing.T) {
	// The base36 hash of a fixed input must be stable.
	h1 := hash("hello")
	h2 := hash("hello")
	if h1 != h2 {
		t.Errorf("hash not stable: %q vs %q", h1, h2)
	}
	if hash("hello") == hash("world") {
		t.Error("hash collision on distinct inputs")
	}
}

type boundaryValue interface {
	~string
	fmt.Stringer
	encoding.TextMarshaler
	json.Marshaler
	driver.Valuer
	Valid() bool
}

type boundaryPointer[T boundaryValue] interface {
	*T
	encoding.TextUnmarshaler
	json.Unmarshaler
	sql.Scanner
}

func boundaryTest[T boundaryValue, P boundaryPointer[T]](t *testing.T, vec string, parse func(string) (T, error), cast func(string) T) {
	id := cast(vec)
	zero := cast("")

	t.Run("String", func(t *testing.T) {
		if id.String() != vec {
			t.Errorf("String = %q", id.String())
		}
	})

	t.Run("Valid", func(t *testing.T) {
		if !id.Valid() {
			t.Error("Valid() = false")
		}
		if zero.Valid() {
			t.Error("Valid(zero) = true")
		}
	})

	t.Run("ParseRoundTrip", func(t *testing.T) {
		got, err := parse(vec)
		if err != nil || string(got) != vec {
			t.Errorf("parse = %q, %v", got, err)
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
			t.Error("MarshalText(zero) succeeded")
		}
	})

	t.Run("UnmarshalText", func(t *testing.T) {
		var dst T = cast("sentinel")
		var p P = &dst
		if err := p.UnmarshalText([]byte(vec)); err != nil || string(dst) != vec {
			t.Errorf("UnmarshalText = %q, %v", dst, err)
		}
		dst = cast("sentinel")
		if err := p.UnmarshalText([]byte("bad")); err == nil {
			t.Error("UnmarshalText(bad) succeeded")
		} else if string(dst) != "sentinel" {
			t.Errorf("receiver mutated: %q", dst)
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
			t.Error("MarshalJSON(zero) succeeded")
		}
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		var dst T = cast("sentinel")
		var p P = &dst
		if err := p.UnmarshalJSON([]byte(`"` + vec + `"`)); err != nil || string(dst) != vec {
			t.Errorf("UnmarshalJSON = %q, %v", dst, err)
		}
		if err := p.UnmarshalJSON([]byte("null")); err != nil || string(dst) != "" {
			t.Errorf("UnmarshalJSON(null) = %q, %v", dst, err)
		}
		dst = cast("sentinel")
		if err := p.UnmarshalJSON([]byte(`""`)); err == nil {
			t.Error("UnmarshalJSON(\"\") succeeded")
		} else if string(dst) != "sentinel" {
			t.Errorf("receiver mutated: %q", dst)
		}
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
			t.Errorf("receiver mutated: %q", dst)
		}
	})

	t.Run("Value", func(t *testing.T) {
		v, err := id.Value()
		if err != nil || v != vec {
			t.Errorf("Value = %v, %v", v, err)
		}
		v, err = zero.Value()
		if err != nil || v != nil {
			t.Errorf("Value(zero) = %v, %v", v, err)
		}
		if _, err := cast("bad").Value(); err == nil {
			t.Error("Value(invalid) succeeded")
		}
	})
}

func TestBoundaryV1(t *testing.T) {
	boundaryTest[V1[struct{}]](t, vecV1,
		func(s string) (V1[struct{}], error) { return ParseV1[struct{}](s) },
		func(s string) V1[struct{}] { return V1[struct{}](s) },
	)
}

func TestBoundaryV2(t *testing.T) {
	boundaryTest[V2[struct{}]](t, vecV2,
		func(s string) (V2[struct{}], error) { return ParseV2[struct{}](s) },
		func(s string) V2[struct{}] { return V2[struct{}](s) },
	)
}

func TestNewV2(t *testing.T) {
	id := NewV2[struct{}]()
	if !ValidV2(string(id)) {
		t.Errorf("NewV2() = %q, not a valid cuid2", id)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, fmt.Errorf("boom") }

func TestSecureRandomPanic(t *testing.T) {
	old := randReader
	randReader = errReader{}
	defer func() { randReader = old }()

	defer func() {
		if recover() == nil {
			t.Error("secureRandom should panic on entropy failure")
		}
	}()
	secureRandom()
}

func TestInterfaceAssertions(t *testing.T) {
	var (
		_ fmt.Stringer             = V1[struct{}]("")
		_ encoding.TextMarshaler   = V1[struct{}]("")
		_ encoding.TextUnmarshaler = (*V1[struct{}])(nil)
		_ json.Marshaler           = V1[struct{}]("")
		_ json.Unmarshaler         = (*V1[struct{}])(nil)
		_ sql.Scanner              = (*V1[struct{}])(nil)
		_ driver.Valuer            = V1[struct{}]("")
		_ fmt.Stringer             = V2[struct{}]("")
		_ sql.Scanner              = (*V2[struct{}])(nil)
		_ driver.Valuer            = V2[struct{}]("")
	)
	_ = errors.Is
	_ = strings.TrimSpace
}
