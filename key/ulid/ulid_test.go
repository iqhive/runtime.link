package ulid

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

const vecULID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

func TestParseCanonical(t *testing.T) {
	inputs := []string{
		vecULID,
		"01arz3ndektsv4rrffq69g5fav", // lowercase accepted
		"01ArZ3nDeKtSv4RrFfQ69G5fAv", // mixed case accepted
	}
	for _, in := range inputs {
		id, err := Parse[struct{}](in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if string(id) != vecULID {
			t.Errorf("Parse(%q) = %q, want canonical %q", in, id, vecULID)
		}
	}
}

func TestParseMax(t *testing.T) {
	const max = "7ZZZZZZZZZZZZZZZZZZZZZZZZZ"
	id, err := Parse[struct{}](max)
	if err != nil {
		t.Fatalf("Parse(max): %v", err)
	}
	if string(id) != max {
		t.Errorf("Parse(max) = %q", id)
	}
}

func TestParseInvalid(t *testing.T) {
	bad := []string{
		"",
		"01ARZ3NDEKTSV4RRFFQ69G5FA",   // 25 chars
		"01ARZ3NDEKTSV4RRFFQ69G5FAVV", // 27 chars
		"8ZZZZZZZZZZZZZZZZZZZZZZZZZ",  // overflow: first char > 7
		"01ARZ3NDEKTSV4RRFFQ69G5FAI",  // contains excluded I
		"01ARZ3NDEKTSV4RRFFQ69G5FAL",  // contains excluded L
		"01ARZ3NDEKTSV4RRFFQ69G5FAO",  // contains excluded O
		"01ARZ3NDEKTSV4RRFFQ69G5FAU",  // contains excluded U
		"01ARZ3NDEKTSV4RRFFQ69G5FA-",  // invalid char
		"X1ARZ3NDEKTSV4RRFFQ69G5FAV",  // overflow: valid crockford char > '7'
		" 1ARZ3NDEKTSV4RRFFQ69G5FAV",  // invalid first character (space)
	}
	for _, s := range bad {
		if _, err := Parse[struct{}](s); err == nil {
			t.Errorf("Parse(%q) succeeded", s)
		}
	}
}

func TestValid(t *testing.T) {
	if !Valid(vecULID) {
		t.Error("Valid(ulid) = false")
	}
	if Valid("") {
		t.Error("Valid(\"\") = true")
	}
	if Valid("8ZZZZZZZZZZZZZZZZZZZZZZZZZ") {
		t.Error("Valid(overflow) = true")
	}
}

func TestNew(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		id := New[struct{}]()
		s := string(id)
		if len(s) != 26 {
			t.Fatalf("New() length = %d", len(s))
		}
		if !Valid(s) {
			t.Fatalf("New() = %q, not valid", s)
		}
		if seen[s] {
			t.Fatalf("duplicate ulid: %q", s)
		}
		seen[s] = true
	}
}

func TestNewMonotonic(t *testing.T) {
	const g = 8
	const per = 2000
	results := make([][]V1[struct{}], g)
	var wg sync.WaitGroup
	for i := 0; i < g; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = make([]V1[struct{}], per)
			for j := 0; j < per; j++ {
				results[i][j] = New[struct{}]()
			}
		}(i)
	}
	wg.Wait()
	for i := 0; i < g; i++ {
		for j := 1; j < per; j++ {
			if string(results[i][j-1]) >= string(results[i][j]) {
				t.Fatalf("ulid not monotonic in goroutine %d at %d: %q >= %q",
					i, j, results[i][j-1], results[i][j])
			}
		}
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestNewWithError(t *testing.T) {
	if _, err := NewWith[struct{}](time.Now().Truncate(0), errReader{}); err == nil {
		t.Error("NewWith with failing entropy should error")
	}
}

func TestNewWithDeterministic(t *testing.T) {
	// A fixed entropy source must produce a reproducible id for a fixed time.
	ent := strings.NewReader("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	id1, err := NewWith[struct{}](time.Unix(1_700_000_000, 0), ent)
	if err != nil {
		t.Fatal(err)
	}
	ent2 := strings.NewReader("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	id2, err := NewWith[struct{}](time.Unix(1_700_000_000, 0), ent2)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Errorf("deterministic NewWith produced different ids: %q vs %q", id1, id2)
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
	id := cast(vecULID)
	zero := cast("")

	t.Run("String", func(t *testing.T) {
		if id.String() != vecULID {
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
		got, err := parse(vecULID)
		if err != nil || string(got) != vecULID {
			t.Errorf("parse = %q, %v", got, err)
		}
		if _, err := parse(""); err == nil {
			t.Error("parse(\"\") succeeded")
		}
	})

	t.Run("MarshalText", func(t *testing.T) {
		b, err := id.MarshalText()
		if err != nil || string(b) != vecULID {
			t.Errorf("MarshalText = %q, %v", b, err)
		}
		if _, err := zero.MarshalText(); err == nil {
			t.Error("MarshalText(zero) succeeded")
		}
	})

	t.Run("UnmarshalText", func(t *testing.T) {
		var dst T = cast("sentinel")
		var p P = &dst
		if err := p.UnmarshalText([]byte(vecULID)); err != nil || string(dst) != vecULID {
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
		if err := json.Unmarshal(b, &back); err != nil || back != vecULID {
			t.Errorf("JSON round trip = %q, %v", back, err)
		}
		if _, err := zero.MarshalJSON(); err == nil {
			t.Error("MarshalJSON(zero) succeeded")
		}
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		var dst T = cast("sentinel")
		var p P = &dst
		if err := p.UnmarshalJSON([]byte(`"` + vecULID + `"`)); err != nil || string(dst) != vecULID {
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
		if err := p.Scan(vecULID); err != nil || string(dst) != vecULID {
			t.Errorf("Scan(string) = %q, %v", dst, err)
		}
		if err := p.Scan([]byte(vecULID)); err != nil || string(dst) != vecULID {
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
		if err != nil || v != vecULID {
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

func TestGenerateMonotonicRandFailure(t *testing.T) {
	// Reset generator state so the fresh-entropy path (which reads from the
	// entropy source) is taken rather than the increment path.
	genMu.Lock()
	genLastMs = 0
	genLastEnt = [10]byte{}
	genLast = [16]byte{}
	genMu.Unlock()

	old := randEntropy
	randEntropy = errReader{}
	defer func() { randEntropy = old }()

	if _, err := generateMonotonic(time.Now()); err == nil {
		t.Error("generateMonotonic should fail when entropy source fails")
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

func TestGenerateMonotonicOverflow(t *testing.T) {
	genMu.Lock()
	genLastMs = msOf(time.Now())
	for i := range genLastEnt {
		genLastEnt[i] = 0xff
	}
	genMu.Unlock()

	if _, err := generateMonotonic(time.Now()); err == nil {
		t.Error("generateMonotonic should report overflow when entropy is exhausted")
	}

	genMu.Lock()
	genLastMs = 0
	genLastEnt = [10]byte{}
	genLast = [16]byte{}
	genMu.Unlock()
}

func TestInc128Wrap(t *testing.T) {
	var all [16]byte
	for i := range all {
		all[i] = 0xff
	}
	if got := inc128(all); got != ([16]byte{}) {
		t.Errorf("inc128(max) = %x, want wrap to zero", got)
	}
}
