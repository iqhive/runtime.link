package nanoid

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const vecNano = "V1StGXR8_Z5jdHi6B-myT"

func TestNew(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 5000; i++ {
		id := New[struct{}]()
		s := string(id)
		if len(s) != 21 {
			t.Fatalf("New() length = %d, want 21", len(s))
		}
		if !Valid(s) {
			t.Fatalf("New() = %q, not valid", s)
		}
		if seen[s] {
			t.Fatalf("duplicate nanoid: %q", s)
		}
		seen[s] = true
	}
}

func TestParseRoundTrip(t *testing.T) {
	id, err := Parse[struct{}](vecNano)
	if err != nil {
		t.Fatalf("Parse(%q): %v", vecNano, err)
	}
	if string(id) != vecNano {
		t.Errorf("Parse = %q", id)
	}
}

func TestParseInvalid(t *testing.T) {
	bad := []string{
		"",
		"V1StGXR8_Z5jdHi6B-my",        // 20 chars
		"V1StGXR8_Z5jdHi6B-myTT",      // 22 chars
		"V1StGXR8_Z5jdHi6B-myT!",      // invalid char !
		"V1StGXR8_Z5jdHi6B-myT ",      // invalid char space
		"V1StGXR8_Z5jdHi6B-myT\u00e9", // non-ASCII
		"!1StGXR8_Z5jdHi6B-myT",       // invalid char, correct length
	}
	for _, s := range bad {
		if _, err := Parse[struct{}](s); err == nil {
			t.Errorf("Parse(%q) succeeded", s)
		}
	}
}

func TestValid(t *testing.T) {
	if !Valid(vecNano) {
		t.Error("Valid(nanoid) = false")
	}
	if Valid("") {
		t.Error("Valid(\"\") = true")
	}
	if Valid("V1StGXR8_Z5jdHi6B-myT!") {
		t.Error("Valid(bad char) = true")
	}
}

func TestNewWithDeterministic(t *testing.T) {
	// A fixed entropy byte stream must produce a reproducible id.
	ent1 := strings.NewReader("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	id1, err := NewWith[struct{}](ent1)
	if err != nil {
		t.Fatal(err)
	}
	ent2 := strings.NewReader("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	id2, err := NewWith[struct{}](ent2)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Errorf("deterministic NewWith produced different ids: %q vs %q", id1, id2)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, fmt.Errorf("boom") }

func TestGenerateRandFailure(t *testing.T) {
	old := randEntropy
	randEntropy = errReader{}
	defer func() { randEntropy = old }()

	if _, err := generate(randEntropy, 21); err == nil {
		t.Error("generate should fail when entropy source fails")
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Error("New should panic on entropy failure")
			}
		}()
		_ = New[struct{}]()
	}()
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
	id := cast(vecNano)
	zero := cast("")

	t.Run("String", func(t *testing.T) {
		if id.String() != vecNano {
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
		got, err := parse(vecNano)
		if err != nil || string(got) != vecNano {
			t.Errorf("parse = %q, %v", got, err)
		}
		if _, err := parse(""); err == nil {
			t.Error("parse(\"\") succeeded")
		}
	})

	t.Run("MarshalText", func(t *testing.T) {
		b, err := id.MarshalText()
		if err != nil || string(b) != vecNano {
			t.Errorf("MarshalText = %q, %v", b, err)
		}
		if _, err := zero.MarshalText(); err == nil {
			t.Error("MarshalText(zero) succeeded")
		}
	})

	t.Run("UnmarshalText", func(t *testing.T) {
		var dst T = cast("sentinel")
		var p P = &dst
		if err := p.UnmarshalText([]byte(vecNano)); err != nil || string(dst) != vecNano {
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
		if err := json.Unmarshal(b, &back); err != nil || back != vecNano {
			t.Errorf("JSON round trip = %q, %v", back, err)
		}
		if _, err := zero.MarshalJSON(); err == nil {
			t.Error("MarshalJSON(zero) succeeded")
		}
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		var dst T = cast("sentinel")
		var p P = &dst
		if err := p.UnmarshalJSON([]byte(`"` + vecNano + `"`)); err != nil || string(dst) != vecNano {
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
		if err := p.Scan(vecNano); err != nil || string(dst) != vecNano {
			t.Errorf("Scan(string) = %q, %v", dst, err)
		}
		if err := p.Scan([]byte(vecNano)); err != nil || string(dst) != vecNano {
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
		if err != nil || v != vecNano {
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
