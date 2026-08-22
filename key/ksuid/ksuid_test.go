package ksuid

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

// Reference strings from the segmentio/ksuid specification.
const (
	vecMin = "000000000000000000000000000"
	vecMax = "aWgEPTl1tmebfsQzFP4bxwgy80V"
	vecEx  = "0ujsswThIGTUYm2K8FjOOfXtY1K"
)

func TestParseRoundTrip(t *testing.T) {
	for _, in := range []string{vecMin, vecMax, vecEx} {
		id, err := Parse[struct{}](in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if string(id) != in {
			t.Errorf("Parse(%q) = %q, want canonical %q", in, id, in)
		}
	}
}

func TestParseMinMaxBytes(t *testing.T) {
	id, err := Parse[struct{}](vecMin)
	if err != nil {
		t.Fatal(err)
	}
	b, err := decodeBase62(vecMin)
	if err != nil {
		t.Fatal(err)
	}
	if b != ([20]byte{}) {
		t.Errorf("min decodes to %x, want all zero", b)
	}
	_ = id

	b, err = decodeBase62(vecMax)
	if err != nil {
		t.Fatal(err)
	}
	all := [20]byte{}
	for i := range all {
		all[i] = 0xff
	}
	if b != all {
		t.Errorf("max decodes to %x, want all 0xff", b)
	}
}

func TestParseInvalid(t *testing.T) {
	bad := []string{
		"",
		"00000000000000000000000000",   // 26 chars
		"0000000000000000000000000000", // 28 chars
		"00000000000000000000000000!",  // invalid char
		"000000000000000000000000000-", // invalid char
		"zzzzzzzzzzzzzzzzzzzzzzzzzzz",  // overflow: > 2^160
	}
	for _, s := range bad {
		if _, err := Parse[struct{}](s); err == nil {
			t.Errorf("Parse(%q) succeeded", s)
		}
	}
}

func TestValid(t *testing.T) {
	if !Valid(vecEx) {
		t.Error("Valid(ex) = false")
	}
	if Valid("") {
		t.Error("Valid(\"\") = true")
	}
	if Valid("zzzzzzzzzzzzzzzzzzzzzzzzzzz") {
		t.Error("Valid(overflow) = true")
	}
}

func TestNew(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		id := New[struct{}]()
		s := string(id)
		if len(s) != 27 {
			t.Fatalf("New() length = %d", len(s))
		}
		if !Valid(s) {
			t.Fatalf("New() = %q, not valid", s)
		}
		if seen[s] {
			t.Fatalf("duplicate ksuid: %q", s)
		}
		seen[s] = true
	}
}

func TestNewFromParts(t *testing.T) {
	payload := []byte("0123456789abcdef") // 16 bytes
	id, err := NewFromParts[struct{}](time.Unix(1_500_000_000, 0), payload)
	if err != nil {
		t.Fatal(err)
	}
	if !Valid(string(id)) {
		t.Fatalf("NewFromParts = %q, not valid", id)
	}
	// deterministic
	id2, err := NewFromParts[struct{}](time.Unix(1_500_000_000, 0), payload)
	if err != nil {
		t.Fatal(err)
	}
	if id != id2 {
		t.Errorf("NewFromParts not deterministic: %q vs %q", id, id2)
	}
	// wrong payload size
	if _, err := NewFromParts[struct{}](time.Now(), []byte("short")); err == nil {
		t.Error("NewFromParts with short payload should fail")
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

func boundaryTest[T boundaryValue, P boundaryPointer[T]](t *testing.T, parse func(string) (T, error), cast func(string) T) {
	id := cast(vecEx)
	zero := cast("")

	t.Run("String", func(t *testing.T) {
		if id.String() != vecEx {
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
		got, err := parse(vecEx)
		if err != nil || string(got) != vecEx {
			t.Errorf("parse = %q, %v", got, err)
		}
		if _, err := parse(""); err == nil {
			t.Error("parse(\"\") succeeded")
		}
	})

	t.Run("MarshalText", func(t *testing.T) {
		b, err := id.MarshalText()
		if err != nil || string(b) != vecEx {
			t.Errorf("MarshalText = %q, %v", b, err)
		}
		if _, err := zero.MarshalText(); err == nil {
			t.Error("MarshalText(zero) succeeded")
		}
	})

	t.Run("UnmarshalText", func(t *testing.T) {
		var dst T = cast("sentinel")
		var p P = &dst
		if err := p.UnmarshalText([]byte(vecEx)); err != nil || string(dst) != vecEx {
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
		if err := json.Unmarshal(b, &back); err != nil || back != vecEx {
			t.Errorf("JSON round trip = %q, %v", back, err)
		}
		if _, err := zero.MarshalJSON(); err == nil {
			t.Error("MarshalJSON(zero) succeeded")
		}
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		var dst T = cast("sentinel")
		var p P = &dst
		if err := p.UnmarshalJSON([]byte(`"` + vecEx + `"`)); err != nil || string(dst) != vecEx {
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
		if err := p.Scan(vecEx); err != nil || string(dst) != vecEx {
			t.Errorf("Scan(string) = %q, %v", dst, err)
		}
		if err := p.Scan([]byte(vecEx)); err != nil || string(dst) != vecEx {
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
		if err != nil || v != vecEx {
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

func TestBoundary(t *testing.T) {
	boundaryTest[V1[struct{}]](t,
		func(s string) (V1[struct{}], error) { return Parse[struct{}](s) },
		func(s string) V1[struct{}] { return V1[struct{}](s) },
	)
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, fmt.Errorf("boom") }

func TestGenerateRandFailure(t *testing.T) {
	old := randEntropy
	randEntropy = errReader{}
	defer func() { randEntropy = old }()

	if _, err := generate(time.Now()); err == nil {
		t.Error("generate should fail when entropy source fails")
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Error("mustGenerate should panic on entropy failure")
			}
		}()
		mustGenerate(time.Now())
	}()
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
	)
	_ = errors.Is
	_ = strings.TrimSpace
}
