// Package ulid provides a production-ready ULID (Universally Unique
// Lexicographically Sortable Identifier) type.
//
// A ULID is a 128-bit value: a 48-bit Unix-millisecond timestamp followed by
// 80 bits of cryptographically secure entropy, encoded into 26 characters of
// Crockford Base32 (see https://github.com/ulid/spec). Because the timestamp
// occupies the most significant bits, ULIDs sort lexicographically by
// generation time, which makes them good database primary keys.
//
// The V1[T] type is string-backed so it remains comparable, usable as a map
// key, and convertible to its canonical text form. Generation is monotonic
// and concurrency-safe: ids produced within the same millisecond are strictly
// increasing, matching the reference implementation's guarantee.
//
// This package has no third-party dependencies; entropy comes from
// crypto/rand.
package ulid

import (
	"bytes"
	"crypto/rand"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	ident "runtime.link/key/internal/id"
)

// Encoding is the Crockford Base32 alphabet used by ULIDs. It deliberately
// excludes the letters I, L, O, and U to avoid confusion with digits and
// with each other.
const Encoding = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// EncodedSize is the fixed length of the canonical text form of a ULID.
const EncodedSize = 26

var (
	// ErrSize is returned when a ULID string has the wrong length.
	ErrSize = errors.New("ulid: bad data size")

	// ErrCharacters is returned when a ULID string contains characters
	// outside the Crockford Base32 alphabet.
	ErrCharacters = errors.New("ulid: bad data characters")

	// ErrOverflow is returned when a ULID string encodes a value larger
	// than 128 bits (its first character exceeds '7').
	ErrOverflow = errors.New("ulid: overflow")

	// ErrEntropy is returned when the entropy source fails.
	ErrEntropy = errors.New("ulid: entropy source failed")
)

// V1 is a ULID identifier for a value of type T.
type V1[T any] string

var (
	genMu      sync.Mutex
	genLastMs  uint64
	genLastEnt [10]byte
	genLast    [16]byte
)

// randEntropy is the entropy source used by monotonic generation. It is a
// package variable so tests can inject a failing source to exercise the error
// and panic paths; production always uses crypto/rand.
var randEntropy io.Reader = rand.Reader

// New returns a new, monotonically increasing ULID identifier for the current
// time. It is safe for concurrent use.
//
// It panics only if the system's cryptographically secure random source is
// unavailable, matching the convention of the standard library's uuid.New.
func New[T any]() V1[T] {
	return V1[T](mustGenerate(time.Now()))
}

// NewWith returns a ULID identifier for the given time, reading exactly 80
// bits of entropy from the provided source. It is deterministic: identical
// (time, entropy) inputs always produce the identical id, and it does not
// consult or mutate the monotonic generator's state. It is intended for
// testing and for callers injecting their own entropy.
func NewWith[T any](now time.Time, entropy io.Reader) (V1[T], error) {
	s, err := generate(now, entropy)
	return V1[T](s), err
}

// Parse parses a string as a ULID identifier, returning its canonical
// uppercase text form. Lowercase and mixed-case input are accepted.
func Parse[T any](s string) (V1[T], error) {
	text, err := parse(s)
	return V1[T](text), err
}

// Valid reports whether s is a well-formed ULID.
func Valid(s string) bool { _, err := parse(s); return err == nil }

// mustGenerate generates a monotonic ULID, panicking on entropy failure.
func mustGenerate(now time.Time) string {
	s, err := generateMonotonic(now)
	if err != nil {
		panic(err)
	}
	return s
}

// generate produces the deterministic canonical text of a ULID for the given
// time and entropy source, without touching the monotonic generator state.
// entropy must not be nil.
func generate(now time.Time, entropy io.Reader) (string, error) {
	ms := msOf(now)
	var ent [10]byte
	if _, err := io.ReadFull(entropy, ent[:]); err != nil {
		return "", ident.Wrap("ulid", ErrEntropy)
	}
	return encode(build(ms, ent)), nil
}

// generateMonotonic produces a ULID for the current time, guaranteeing that
// ids across all callers and goroutines are strictly increasing. Entropy is
// drawn fresh from crypto/rand when the millisecond advances, otherwise the
// previous entropy is incremented.
//
// The wall clock is not guaranteed to be consistent across goroutines (a
// sub-millisecond call can read M while another reads M+1), so in addition to
// the increment path we enforce a hard invariant: the emitted 128-bit value is
// always strictly greater than the previously emitted value. This makes the
// ordering guarantee robust to clock jitter.
func generateMonotonic(now time.Time) (string, error) {
	ms := msOf(now)
	genMu.Lock()
	defer genMu.Unlock()

	var ent [10]byte
	if ms == genLastMs {
		if !inc80(genLastEnt[:]) {
			return "", ident.Wrap("ulid", ErrOverflow)
		}
		copy(ent[:], genLastEnt[:])
	} else {
		if _, err := io.ReadFull(randEntropy, ent[:]); err != nil {
			return "", ident.Wrap("ulid", ErrEntropy)
		}
		copy(genLastEnt[:], ent[:])
		genLastMs = ms
	}

	cand := build(ms, ent)
	if bytes.Compare(cand[:], genLast[:]) <= 0 {
		cand = inc128(genLast)
	}
	genLast = cand
	return encode(cand), nil
}

// msOf returns the unix milliseconds of a time.
func msOf(t time.Time) uint64 {
	return uint64(t.Unix())*1000 + uint64(t.Nanosecond()/int(time.Millisecond))
}

// build lays out a ULID's 16 bytes: a 48-bit big-endian millisecond timestamp
// followed by exactly 10 bytes of entropy.
func build(ms uint64, ent [10]byte) [16]byte {
	var b [16]byte
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	copy(b[6:], ent[:])
	return b
}

// inc80 increments an 80-bit big-endian value by one, returning false if it
// overflows (all bits set).
func inc80(b []byte) bool {
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] < 0xff {
			b[i]++
			return true
		}
		b[i] = 0
	}
	return false
}

// inc128 increments a 128-bit big-endian value by one, wrapping to zero on
// overflow (which cannot be reached in realistic use).
func inc128(b [16]byte) [16]byte {
	for i := 15; i >= 0; i-- {
		if b[i] < 0xff {
			b[i]++
			return b
		}
		b[i] = 0
	}
	return b
}

// parse validates and canonicalizes a ULID string.
func parse(s string) (string, error) {
	b, err := decode(s)
	if err != nil {
		return "", ident.Wrap("ulid", err)
	}
	return encode(b), nil
}

// encode renders a 16-byte ULID value as its canonical 26-character
// Crockford Base32 text.
//
// A ULID is 128 bits of data encoded into 26 characters of 5 bits each
// (130 bits). The two spare bits are leading padding at the top of the first
// character, so the first character carries only 3 data bits and must be in
// 0..7. The remaining 25 characters carry 5 data bits each.
func encode(b [16]byte) string {
	out := make([]byte, 0, EncodedSize)
	out = append(out, Encoding[getBits(b[:], 0, 3)])
	for i := 1; i < EncodedSize; i++ {
		out = append(out, Encoding[getBits(b[:], 3+5*(i-1), 5)])
	}
	return string(out)
}

// decode parses a 26-character ULID string into its 16-byte value, validating
// length, alphabet, and the 128-bit overflow bound.
func decode(s string) ([16]byte, error) {
	var b [16]byte
	if len(s) != EncodedSize {
		return b, ErrSize
	}
	if s[0] > '7' {
		return b, ErrOverflow
	}
	v, ok := dec(s[0])
	if !ok {
		return b, ErrCharacters
	}
	setBits(b[:], 0, 3, int(v))
	for i := 1; i < EncodedSize; i++ {
		v, ok := dec(s[i])
		if !ok {
			return b, ErrCharacters
		}
		setBits(b[:], 3+5*(i-1), 5, int(v))
	}
	return b, nil
}

// getBits reads n bits (MSB-first) from the 128-bit value starting at bit
// position pos.
func getBits(b []byte, pos, n int) int {
	var v int
	for i := 0; i < n; i++ {
		p := pos + i
		v = (v << 1) | int((b[p/8]>>uint(7-(p%8)))&1)
	}
	return v
}

// setBits writes the low n bits of v into the 128-bit value at bit position
// pos (MSB-first).
func setBits(b []byte, pos, n, v int) {
	for i := n - 1; i >= 0; i-- {
		p := pos + i
		bit := uint(7 - (p % 8))
		if v&1 == 1 {
			b[p/8] |= 1 << bit
		} else {
			b[p/8] &^= 1 << bit
		}
		v >>= 1
	}
}

// dec maps a Crockford Base32 character to its value, accepting both cases.
// It returns false for characters outside the alphabet (including the excluded
// letters I, L, O, and U).
func dec(c byte) (byte, bool) {
	if c >= '0' && c <= '9' {
		return c - '0', true
	}
	if c >= 'a' && c <= 'z' {
		c -= 32
	}
	switch c {
	case 'A':
		return 10, true
	case 'B':
		return 11, true
	case 'C':
		return 12, true
	case 'D':
		return 13, true
	case 'E':
		return 14, true
	case 'F':
		return 15, true
	case 'G':
		return 16, true
	case 'H':
		return 17, true
	case 'J':
		return 18, true
	case 'K':
		return 19, true
	case 'M':
		return 20, true
	case 'N':
		return 21, true
	case 'P':
		return 22, true
	case 'Q':
		return 23, true
	case 'R':
		return 24, true
	case 'S':
		return 25, true
	case 'T':
		return 26, true
	case 'V':
		return 27, true
	case 'W':
		return 28, true
	case 'X':
		return 29, true
	case 'Y':
		return 30, true
	case 'Z':
		return 31, true
	}
	return 0, false
}

// marshalText returns the canonical text of id, rejecting the zero value.
func marshalText[T ~string](id T) ([]byte, error) {
	if id == "" {
		return nil, ident.Wrap("ulid", ident.ErrInvalid)
	}
	return []byte(id), nil
}

// unmarshalText validates b as a ULID and stores its canonical form.
func unmarshalText[T ~string](dst *T, b []byte) error {
	text, err := parse(string(b))
	if err != nil {
		return err
	}
	*dst = T(text)
	return nil
}

// unmarshalJSON decodes a JSON string ULID. JSON null clears the receiver.
func unmarshalJSON[T ~string](dst *T, b []byte) error {
	if string(b) == "null" {
		*dst = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return ident.Wrap("ulid", err)
	}
	return unmarshalText(dst, []byte(s))
}

// scan reads a database/sql source value into dst.
func scan[T ~string](dst *T, src any) error {
	s, err := ident.ScanText(src)
	if err != nil {
		return err
	}
	if s == "" {
		*dst = ""
		return nil
	}
	return unmarshalText(dst, []byte(s))
}

// value converts id into a database/sql driver value.
func value[T ~string](id T) (driver.Value, error) {
	if id == "" {
		return nil, nil
	}
	if _, err := parse(string(id)); err != nil {
		return nil, err
	}
	return string(id), nil
}

func (id V1[T]) String() string               { return string(id) }
func (id V1[T]) Valid() bool                  { return Valid(string(id)) }
func (id V1[T]) MarshalText() ([]byte, error) { return marshalText(id) }
func (id *V1[T]) UnmarshalText(b []byte) error {
	return unmarshalText(id, b)
}
func (id V1[T]) MarshalJSON() ([]byte, error)  { return ident.MarshalJSON(string(id)) }
func (id *V1[T]) UnmarshalJSON(b []byte) error { return unmarshalJSON(id, b) }
func (id *V1[T]) Scan(src any) error           { return scan(id, src) }
func (id V1[T]) Value() (driver.Value, error)  { return value(id) }
