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

	ident "github.com/iqhive/runtime.link/key/internal/id"
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
	if err := jd.UnmarshalJSON([]byte(`"` + text + `"`)); err != nil || jd != id {
		t.Errorf("UnmarshalJSON(quoted) = %d, %v", jd, err)
	}
	for _, j := range []string{`"abc"`, `""`, `{}`, `[]`, `true`, `1.5`} {
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

func TestRejectNegativeIDs(t *testing.T) {
	for _, number := range []int64{-1, -9223372036854775808} {
		text := strconv.FormatInt(number, 10)
		t.Run(text, func(t *testing.T) {
			for name, decode := range map[string]func(*FlakeID[struct{}]) error{
				"JSON":      func(id *FlakeID[struct{}]) error { return json.Unmarshal([]byte(text), id) },
				"ScanInt":   func(id *FlakeID[struct{}]) error { return id.Scan(number) },
				"ScanText":  func(id *FlakeID[struct{}]) error { return id.Scan(text) },
				"ScanBytes": func(id *FlakeID[struct{}]) error { return id.Scan([]byte(text)) },
			} {
				t.Run(name, func(t *testing.T) {
					id := FlakeID[struct{}](123)
					if err := decode(&id); err == nil {
						t.Error("accepted negative ID")
					}
					if id != 123 {
						t.Errorf("receiver changed on error: %d", id)
					}
				})
			}
			id := FlakeID[struct{}](number)
			if id.Valid() {
				t.Error("negative ID is valid")
			}
			if _, err := id.MarshalText(); err == nil {
				t.Error("MarshalText accepted negative ID")
			}
			if _, err := id.MarshalJSON(); err == nil {
				t.Error("MarshalJSON accepted negative ID")
			}
			if _, err := id.Value(); err == nil {
				t.Error("Value accepted negative ID")
			}
		})
	}
}

func workerOf[T any](id FlakeID[T]) int64 {
	return (int64(id) >> sequenceBits) & (1<<workerBits - 1)
}

func TestNewStaysIncreasingAfterClockRollback(t *testing.T) {
	// A last timestamp an hour ahead of the clock is what a rollback leaves
	// behind. Enough ids are drawn to wrap the 12-bit sequence as well.
	genMu.Lock()
	genLastMs = time.Now().UnixMilli() - epoch + time.Hour.Milliseconds()
	genMu.Unlock()
	prev := New[struct{}]()
	for range 3 << sequenceBits {
		id := New[struct{}]()
		if id <= prev {
			t.Fatalf("not increasing after rollback: %d then %d", prev, id)
		}
		prev = id
	}
}

func TestNewWithUsesTheGivenWorker(t *testing.T) {
	for _, worker := range []int64{0, 1, 512, 1023} {
		id, err := NewWith[struct{}](worker)
		if err != nil {
			t.Fatalf("NewWith(%d): %v", worker, err)
		}
		if !id.Valid() || workerOf(id) != worker {
			t.Errorf("NewWith(%d) = %d with worker %d", worker, int64(id), workerOf(id))
		}
	}
}

func TestNewWithRejectsWorkerOutOfRange(t *testing.T) {
	for _, worker := range []int64{-1, 1024, 1 << 40} {
		id, err := NewWith[struct{}](worker)
		if !errors.Is(err, ErrWorker) || !errors.Is(err, ident.ErrInvalid) {
			t.Errorf("NewWith(%d) = %d, %v, want ErrWorker", worker, int64(id), err)
		}
		if id != 0 {
			t.Errorf("NewWith(%d) returned an id alongside its error: %d", worker, int64(id))
		}
	}
}

func TestNewWithIncreasesForOneWorkerAfterClockRollback(t *testing.T) {
	genMu.Lock()
	genLastMs = time.Now().UnixMilli() - epoch + time.Hour.Milliseconds()
	genMu.Unlock()
	prev, err := NewWith[struct{}](7)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 << sequenceBits {
		id, err := NewWith[struct{}](7)
		if err != nil {
			t.Fatal(err)
		}
		if id <= prev {
			t.Fatalf("not increasing after rollback: %d then %d", prev, id)
		}
		prev = id
	}
}

func TestNewAndNewWithNeverRepeatInAProcess(t *testing.T) {
	// Asking NewWith for New's own worker is the one way the two could collide
	// if they did not share a generator.
	worker := workerOf(New[struct{}]())
	seen := map[FlakeID[struct{}]]bool{}
	for range 5000 {
		id, err := NewWith[struct{}](worker)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range []FlakeID[struct{}]{id, New[struct{}]()} {
			if seen[v] {
				t.Fatalf("repeated id %d", int64(v))
			}
			seen[v] = true
		}
	}
}

func TestGenerateCheckedRejectsTimePastTheLastTimestamp(t *testing.T) {
	genMu.Lock()
	lastMs, seq := genLastMs, genSeq
	genMu.Unlock()
	t.Cleanup(func() {
		genMu.Lock()
		defer genMu.Unlock()
		genLastMs, genSeq = lastMs, seq
	})

	last := time.UnixMilli(epoch + maxTimestamp)
	v, err := generateChecked(last, 1)
	if err != nil || v <= 0 {
		t.Errorf("generateChecked(last timestamp) = %d, %v", v, err)
	}
	past := last.Add(time.Millisecond)
	if v, err := generateChecked(past, 1); !errors.Is(err, ErrOverflow) || v != 0 {
		t.Errorf("generateChecked(past the end) = %d, %v, want ErrOverflow", v, err)
	}
}

// A quoted negative is refused like a bare one (see TestRejectNegativeIDs).
func TestNegativeTextIsRejected(t *testing.T) {
	const sentinel = FlakeID[struct{}](7)
	id := sentinel
	if err := id.UnmarshalJSON([]byte(`"-5"`)); err == nil || id != sentinel {
		t.Errorf("UnmarshalJSON(\"-5\") = %d, %v", int64(id), err)
	}
	for _, src := range []any{"-5", []byte("-5")} {
		id := sentinel
		if err := id.Scan(src); err == nil || id != sentinel {
			t.Errorf("Scan(%T) = %d, %v", src, int64(id), err)
		}
	}
}

func TestScanTreatsEmptyLikeNull(t *testing.T) {
	for _, src := range []any{"", []byte{}, []byte(nil)} {
		id := FlakeID[struct{}](7)
		if err := id.Scan(src); err != nil || id != 0 {
			t.Errorf("Scan(%#v) = %d, %v, want 0, nil", src, int64(id), err)
		}
	}
}

func TestScanTreatsZeroAlikeForEveryDriverForm(t *testing.T) {
	// One BIGINT 0 reaches Scan as int64 or as text depending on the driver's
	// protocol, so the two forms have to agree.
	for _, src := range []any{int64(0), "0", []byte("0")} {
		id := FlakeID[struct{}](7)
		if err := id.Scan(src); err != nil || id != 0 {
			t.Errorf("Scan(%#v) = %d, %v, want 0, nil", src, int64(id), err)
		}
	}
}

func TestParseRoundTripsCanonicalText(t *testing.T) {
	for _, s := range []string{"1", "5", "9223372036854775807"} {
		if id, err := Parse[struct{}](s); err != nil || id.String() != s {
			t.Errorf("Parse(%q) = %d, %v", s, int64(id), err)
		}
	}
}

func TestParseFailuresMatchErrInvalid(t *testing.T) {
	cases := []struct {
		in   string
		want error
	}{
		{"", ident.ErrInvalid},
		{"-5", ident.ErrInvalid},
		{"abc", ErrCharacters},
		{"99999999999999999999", ErrOverflow},
	}
	for _, c := range cases {
		_, err := Parse[struct{}](c.in)
		if !errors.Is(err, c.want) || !errors.Is(err, ident.ErrInvalid) {
			t.Errorf("Parse(%q) = %v, want %v and ErrInvalid", c.in, err, c.want)
		}
	}

	var id FlakeID[struct{}]
	if err := id.UnmarshalJSON([]byte(`true`)); !errors.Is(err, ident.ErrInvalid) {
		t.Errorf("UnmarshalJSON(true) = %v", err)
	}
	if err := id.Scan(1.5); !errors.Is(err, ident.ErrInvalid) {
		t.Errorf("Scan(float64) = %v", err)
	}
}

func TestUnmarshalJSONAcceptsQuotedDecimal(t *testing.T) {
	var id FlakeID[struct{}]
	if err := id.UnmarshalJSON([]byte(`"1234567890123"`)); err != nil || id != 1234567890123 {
		t.Errorf("UnmarshalJSON = %d, %v", int64(id), err)
	}
	for _, j := range []string{`"+5"`, `"0005"`, `" 5"`, `"1.5"`, `"1e3"`, `"99999999999999999999"`} {
		id := FlakeID[struct{}](7)
		if err := id.UnmarshalJSON([]byte(j)); err == nil || id != 7 {
			t.Errorf("UnmarshalJSON(%s) = %d, %v", j, int64(id), err)
		}
	}
}

func TestJSONMapKeyRoundTrip(t *testing.T) {
	in := map[FlakeID[struct{}]]string{1234567890123: "x"}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out map[FlakeID[struct{}]]string
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal(%s): %v", b, err)
	}
	if len(out) != 1 || out[1234567890123] != "x" {
		t.Errorf("round trip of %s = %v", b, out)
	}
}

func TestStringTagReadsQuotedID(t *testing.T) {
	var out struct {
		ID FlakeID[struct{}] `json:"id,string"`
	}
	if err := json.Unmarshal([]byte(`{"id":"1234567890123"}`), &out); err != nil {
		t.Fatal(err)
	}
	if out.ID != 1234567890123 {
		t.Errorf("id = %d", int64(out.ID))
	}
}
