// Package ksuid provides a production-ready KSUID (K-Sortable Unique
// IDentifier) type.
//
// A KSUID is a 20-byte value: a 32-bit big-endian UTC timestamp (seconds
// since the epoch 1_400_000_000) followed by 128 bits of cryptographically
// secure entropy (see https://github.com/segmentio/ksuid). Because the
// timestamp occupies the most significant bits, KSUIDs sort lexicographically
// by generation time. The canonical text form is 27 characters of Base62.
//
// The V1[T] type is string-backed so it remains comparable, usable as a map
// key, and convertible to its canonical text form. This package has no
// third-party dependencies; entropy comes from crypto/rand.
package ksuid

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"time"

	ident "runtime.link/key/internal/id"
)

// epochStamp is the KSUID epoch (May 13, 2014) as a Unix timestamp. Offsetting
// the epoch gives the 32-bit timestamp space a useful lifetime well beyond
// 2106.
const epochStamp int64 = 1_400_000_000

// byteLength is the binary size of a KSUID: 4 timestamp bytes + 16 payload
// bytes.
const byteLength = 20

// stringEncodedLength is the canonical Base62 text length of a KSUID.
const stringEncodedLength = 27

// base62 is the Base62 alphabet used by KSUIDs, ordered so that '0' is the
// least significant digit and 'z' the most significant.
const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var (
	// ErrSize is returned when a KSUID string or payload has the wrong size.
	ErrSize = errors.New("ksuid: bad data size")

	// ErrCharacters is returned when a KSUID string contains characters
	// outside the Base62 alphabet.
	ErrCharacters = errors.New("ksuid: bad data characters")

	// ErrOverflow is returned when a KSUID string encodes a value larger
	// than 160 bits.
	ErrOverflow = errors.New("ksuid: overflow")

	// ErrEntropy is returned when the entropy source fails.
	ErrEntropy = errors.New("ksuid: entropy source failed")
)

// V1 is a KSUID identifier for a value of type T.
type V1[T any] string

// randEntropy is the entropy source for generation. It is a package variable
// so tests can inject a failing source to exercise the error paths; production
// always uses crypto/rand.
var randEntropy io.Reader = rand.Reader

// New returns a new KSUID identifier for the current time, drawing 128 bits
// of entropy from crypto/rand.
//
// It panics only if the system's cryptographically secure random source is
// unavailable, matching the convention of the standard library's uuid.New.
func New[T any]() V1[T] {
	return V1[T](mustGenerate(time.Now()))
}

// NewFromParts returns a KSUID identifier built from an explicit timestamp and
// a 16-byte payload. It is deterministic and is the error-returning
// constructor used for testing and for callers constructing ids from known
// parts.
func NewFromParts[T any](t time.Time, payload []byte) (V1[T], error) {
	if len(payload) != byteLength-4 {
		return "", ident.Wrap("ksuid", ErrSize)
	}
	var b [byteLength]byte
	ts := uint32(t.Unix() - epochStamp)
	binary.BigEndian.PutUint32(b[:4], ts)
	copy(b[4:], payload)
	return V1[T](encodeBase62(b)), nil
}

// Parse parses a string as a KSUID identifier, returning its canonical Base62
// text form.
func Parse[T any](s string) (V1[T], error) {
	text, err := parse(s)
	return V1[T](text), err
}

// Valid reports whether s is a well-formed KSUID.
func Valid(s string) bool { _, err := parse(s); return err == nil }

// mustGenerate generates a KSUID, panicking on entropy failure.
func mustGenerate(now time.Time) string {
	s, err := generate(now)
	if err != nil {
		panic(err)
	}
	return s
}

// generate produces the canonical text of a KSUID for the given time.
func generate(now time.Time) (string, error) {
	var b [byteLength]byte
	if _, err := io.ReadFull(randEntropy, b[4:]); err != nil {
		return "", ident.Wrap("ksuid", ErrEntropy)
	}
	ts := uint32(now.Unix() - epochStamp)
	binary.BigEndian.PutUint32(b[:4], ts)
	return encodeBase62(b), nil
}

// parse validates and canonicalizes a KSUID string.
func parse(s string) (string, error) {
	b, err := decodeBase62(s)
	if err != nil {
		return "", ident.Wrap("ksuid", err)
	}
	return encodeBase62(b), nil
}

// encodeBase62 renders a 20-byte KSUID value as its canonical 27-character
// Base62 text.
func encodeBase62(b [20]byte) string {
	n := new(big.Int).SetBytes(b[:])
	digits := make([]byte, 0, stringEncodedLength)
	var rem big.Int
	base := big.NewInt(62)
	for n.Sign() > 0 {
		n.QuoRem(n, base, &rem)
		digits = append(digits, base62[rem.Int64()])
	}
	for len(digits) < stringEncodedLength {
		digits = append(digits, '0')
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

// decodeBase62 parses a 27-character Base62 string into its 20-byte value,
// validating length, alphabet, and the 160-bit overflow bound.
func decodeBase62(s string) ([20]byte, error) {
	var b [20]byte
	if len(s) != stringEncodedLength {
		return b, ErrSize
	}
	n := new(big.Int)
	for i := 0; i < len(s); i++ {
		v, ok := val62(s[i])
		if !ok {
			return b, ErrCharacters
		}
		n.Mul(n, big.NewInt(62))
		n.Add(n, big.NewInt(int64(v)))
	}
	if n.BitLen() > 160 {
		return b, ErrOverflow
	}
	n.FillBytes(b[:])
	return b, nil
}

// val62 maps a Base62 character to its value.
func val62(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10, true
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 36, true
	}
	return 0, false
}

// marshalText returns the canonical text of id, rejecting the zero value.
func marshalText[T ~string](id T) ([]byte, error) {
	if id == "" {
		return nil, ident.Wrap("ksuid", ident.ErrInvalid)
	}
	return []byte(id), nil
}

// unmarshalText validates b as a KSUID and stores its canonical form.
func unmarshalText[T ~string](dst *T, b []byte) error {
	text, err := parse(string(b))
	if err != nil {
		return err
	}
	*dst = T(text)
	return nil
}

// unmarshalJSON decodes a JSON string KSUID. JSON null clears the receiver.
func unmarshalJSON[T ~string](dst *T, b []byte) error {
	if string(b) == "null" {
		*dst = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return ident.Wrap("ksuid", err)
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
