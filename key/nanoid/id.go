// Package nanoid provides a production-ready Nano ID identifier type.
//
// Nano ID is a tiny, secure, URL-friendly string identifier (see
// https://github.com/ai/nanoid). The default form is 21 characters drawn from
// a 64-character URL-safe alphabet, which packs roughly 126 bits of entropy
// (similar to a UUIDv4).
//
// The V1[T] type is string-backed so it remains comparable, usable as a map
// key, and convertible to its canonical text form. Generation uses unbiased
// rejection sampling over crypto/rand, so symbols are uniform (no modulo
// bias) and unpredictable. There are no third-party dependencies.
package nanoid

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"

	ident "github.com/iqhive/runtime.link/key/internal/id"
)

// URLAlphabet is the default 64-character URL-safe alphabet used by Nano IDs:
// letters, digits, and the - and _ characters, in the upstream order.
const URLAlphabet = "useandom-26T198340PX75pxJACKVERYMINDBUSHWOLF_GQZbfghjklqvwyzrict"

// DefaultSize is the default Nano ID length (21 characters).
const DefaultSize = 21

var (
	// ErrSize is returned when a Nano ID string has the wrong length.
	ErrSize = errors.New("nanoid: bad data size")

	// ErrCharacters is returned when a Nano ID string contains characters
	// outside the URL-safe alphabet.
	ErrCharacters = errors.New("nanoid: bad data characters")

	// ErrEntropy is returned when the entropy source fails.
	ErrEntropy = errors.New("nanoid: entropy source failed")
)

// V1 is a Nano ID identifier for a value of type T.
type V1[T any] string

// randEntropy is the entropy source for generation. It is a package variable
// so tests can inject a failing source to exercise the error paths; production
// always uses crypto/rand.
var randEntropy io.Reader = rand.Reader

// New returns a new 21-character URL-friendly Nano ID identifier.
//
// It panics only if the system's cryptographically secure random source is
// unavailable, matching the convention of the standard library's uuid.New.
func New[T any]() V1[T] {
	s, err := generate(randEntropy, DefaultSize)
	if err != nil {
		panic(err)
	}
	return V1[T](s)
}

// NewWith returns a 21-character Nano ID identifier reading random bytes from
// the provided source. It is deterministic for a fixed byte stream and is the
// error-returning constructor used for testing and for callers that need to
// inject entropy.
func NewWith[T any](entropy io.Reader) (V1[T], error) {
	s, err := generate(entropy, DefaultSize)
	return V1[T](s), err
}

// Parse parses a string as a Nano ID identifier.
func Parse[T any](s string) (V1[T], error) {
	text, err := parse(s)
	return V1[T](text), err
}

// Valid reports whether s is a well-formed default Nano ID.
func Valid(s string) bool { _, err := parse(s); return err == nil }

// generate draws length random characters from URLAlphabet using unbiased
// rejection sampling so that every character is equally likely.
func generate(entropy io.Reader, length int) (string, error) {
	// mask is the smallest value of the form 2^n - 1 that is >= the alphabet
	// size. Masking a random byte's low bits with it yields a candidate in
	// [0, mask]; candidates >= the alphabet size are rejected and re-sampled,
	// eliminating modulo bias.
	//
	// The comparison uses mask+1 (a power of two) so that an alphabet whose
	// size is exactly a power of two (like the 64-character URL alphabet) stops
	// at mask 63 rather than overgrowing to 127.
	mask := 1
	for mask+1 < len(URLAlphabet) {
		mask = (mask << 1) | 1
	}
	out := make([]byte, length)
	var buf [1]byte
	for i := 0; i < length; i++ {
		for {
			if _, err := io.ReadFull(entropy, buf[:]); err != nil {
				return "", ident.Wrap("nanoid", ErrEntropy)
			}
			if v := int(buf[0]) & mask; v < len(URLAlphabet) {
				out[i] = URLAlphabet[v]
				break
			}
		}
	}
	return string(out), nil
}

// parse validates a Nano ID string: exact default length and membership in
// the URL-safe alphabet.
func parse(s string) (string, error) {
	if len(s) != DefaultSize {
		return "", ErrSize
	}
	for i := 0; i < len(s); i++ {
		if !inAlphabet(s[i]) {
			return "", ErrCharacters
		}
	}
	return s, nil
}

// inAlphabet reports whether c is in the URL-safe alphabet.
func inAlphabet(c byte) bool {
	for i := 0; i < len(URLAlphabet); i++ {
		if URLAlphabet[i] == c {
			return true
		}
	}
	return false
}

// marshalText returns the canonical text of id, rejecting the zero value.
func marshalText[T ~string](id T) ([]byte, error) {
	if id == "" {
		return nil, ident.Wrap("nanoid", ident.ErrInvalid)
	}
	return []byte(id), nil
}

// unmarshalText validates b as a Nano ID and stores it.
func unmarshalText[T ~string](dst *T, b []byte) error {
	text, err := parse(string(b))
	if err != nil {
		return err
	}
	*dst = T(text)
	return nil
}

// unmarshalJSON decodes a JSON string Nano ID. JSON null clears the receiver.
func unmarshalJSON[T ~string](dst *T, b []byte) error {
	if string(b) == "null" {
		*dst = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return ident.Wrap("nanoid", err)
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
