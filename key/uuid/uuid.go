// Package uuid provides production-ready RFC 9562 UUID identifier types.
//
// It exposes one branded type per UUID version (V1..V8), each parameterized
// by the entity type T it identifies. The types are string-backed so they
// remain comparable, usable as map keys, and convertible to/from the
// canonical text form that is stored in databases and JSON.
//
// Generation is deliberately limited to the versions the standard library
// implements (V4 and V7). Parsing and validation support all versions V1-V8,
// because checking a version only requires inspecting the RFC version and
// variant bits. We do not invent generators for V1/V2/V3/V5/V6/V8, which
// require node identifiers, namespaces, or custom algorithms.
//
// All parsing/formatting/generation is delegated to the standard library
// uuid package on Go 1.27+ (see the internal stduuid package), so there is no
// third-party UUID dependency.
package uuid

import (
	"database/sql/driver"
	"encoding/json"

	ident "github.com/iqhive/runtime.link/key/internal/id"
	"github.com/iqhive/runtime.link/key/uuid/internal/stduuid"
)

type (
	// V1 is a version 1 (time + node) UUID identifier for a value of type T.
	V1[T any] string

	// V2 is a version 2 (DCE security) UUID identifier for a value of type T.
	V2[T any] string

	// V3 is a version 3 (name-based, MD5) UUID identifier for a value of type T.
	V3[T any] string

	// V4 is a version 4 (random) UUID identifier for a value of type T.
	V4[T any] string

	// V5 is a version 5 (name-based, SHA-1) UUID identifier for a value of type T.
	V5[T any] string

	// V6 is a version 6 (reordered time) UUID identifier for a value of type T.
	V6[T any] string

	// V7 is a version 7 (Unix Epoch time) UUID identifier for a value of type T.
	V7[T any] string

	// V8 is a version 8 (custom) UUID identifier for a value of type T.
	V8[T any] string
)

// New returns a new version 4 UUID identifier. It mirrors the standard
// library's uuid.New, which returns the algorithm recommended for most
// purposes (currently V4).
func New[T any]() V4[T] {
	return V4[T](stduuid.New().String())
}

// NewV4 returns a new version 4 (random) UUID identifier.
func NewV4[T any]() V4[T] {
	return V4[T](stduuid.NewV4().String())
}

// NewV7 returns a new version 7 (time-ordered) UUID identifier. V7 ids sort
// in increasing order of generation time, which is useful for database
// primary keys.
func NewV7[T any]() V7[T] {
	return V7[T](stduuid.NewV7().String())
}

// ParseV1 parses a string as a version 1 UUID identifier.
func ParseV1[T any](s string) (V1[T], error) { t, err := parseVersion(s, 1); return V1[T](t), err }

// ParseV2 parses a string as a version 2 UUID identifier.
func ParseV2[T any](s string) (V2[T], error) { t, err := parseVersion(s, 2); return V2[T](t), err }

// ParseV3 parses a string as a version 3 UUID identifier.
func ParseV3[T any](s string) (V3[T], error) { t, err := parseVersion(s, 3); return V3[T](t), err }

// ParseV4 parses a string as a version 4 UUID identifier.
func ParseV4[T any](s string) (V4[T], error) { t, err := parseVersion(s, 4); return V4[T](t), err }

// ParseV5 parses a string as a version 5 UUID identifier.
func ParseV5[T any](s string) (V5[T], error) { t, err := parseVersion(s, 5); return V5[T](t), err }

// ParseV6 parses a string as a version 6 UUID identifier.
func ParseV6[T any](s string) (V6[T], error) { t, err := parseVersion(s, 6); return V6[T](t), err }

// ParseV7 parses a string as a version 7 UUID identifier.
func ParseV7[T any](s string) (V7[T], error) { t, err := parseVersion(s, 7); return V7[T](t), err }

// ParseV8 parses a string as a version 8 UUID identifier.
func ParseV8[T any](s string) (V8[T], error) { t, err := parseVersion(s, 8); return V8[T](t), err }

// Valid reports whether s is a well-formed UUID of any recognized version
// (V1-V8) with the RFC 9562 variant bits.
func Valid(s string) bool { _, err := parse(s); return err == nil }

// ValidV1 reports whether s is a well-formed version 1 UUID.
func ValidV1(s string) bool { _, err := parseVersion(s, 1); return err == nil }

// ValidV2 reports whether s is a well-formed version 2 UUID.
func ValidV2(s string) bool { _, err := parseVersion(s, 2); return err == nil }

// ValidV3 reports whether s is a well-formed version 3 UUID.
func ValidV3(s string) bool { _, err := parseVersion(s, 3); return err == nil }

// ValidV4 reports whether s is a well-formed version 4 UUID.
func ValidV4(s string) bool { _, err := parseVersion(s, 4); return err == nil }

// ValidV5 reports whether s is a well-formed version 5 UUID.
func ValidV5(s string) bool { _, err := parseVersion(s, 5); return err == nil }

// ValidV6 reports whether s is a well-formed version 6 UUID.
func ValidV6(s string) bool { _, err := parseVersion(s, 6); return err == nil }

// ValidV7 reports whether s is a well-formed version 7 UUID.
func ValidV7(s string) bool { _, err := parseVersion(s, 7); return err == nil }

// ValidV8 reports whether s is a well-formed version 8 UUID.
func ValidV8(s string) bool { _, err := parseVersion(s, 8); return err == nil }

// parse parses s as a UUID of any recognized version and returns its
// canonical lowercase dashed text. It rejects the nil UUID, unrecognized
// versions, and non-RFC variants.
func parse(s string) (string, error) {
	u, err := stduuid.Parse(s)
	if err != nil {
		return "", ident.Wrap("uuid", err)
	}
	if u == (stduuid.UUID{}) {
		return "", ident.Wrap("uuid", ident.ErrInvalid) // nil UUID is not a usable identifier
	}
	if v := u[6] >> 4; v < 1 || v > 8 {
		return "", ident.Wrap("uuid", ident.ErrInvalid)
	}
	if u[8]>>6 != 0b10 {
		return "", ident.Wrap("uuid", ident.ErrInvalid) // RFC 9562 variant bits must be 10
	}
	return u.String(), nil
}

// parseVersion parses s and requires that its RFC version matches version.
func parseVersion(s string, version byte) (string, error) {
	text, err := parse(s)
	if err != nil {
		return "", err
	}
	if versionOf(text) != version {
		return "", ident.Wrap("uuid", ident.ErrInvalid)
	}
	return text, nil
}

// versionOf returns the RFC version nibble of canonical UUID text. In the
// canonical form the first hex digit of the third group (index 14) holds the
// version. Because parse only accepts versions 1-8, this digit is always a
// decimal digit, so no hex decoding is required.
func versionOf(text string) byte { return text[14] - '0' }

// marshalText returns the canonical text of id, rejecting the zero value.
func marshalText[T ~string](id T) ([]byte, error) {
	if id == "" {
		return nil, ident.Wrap("uuid", ident.ErrInvalid)
	}
	return []byte(id), nil
}

// unmarshalText validates b as a UUID of the given version and stores its
// canonical form. The receiver is only mutated on success.
func unmarshalText[T ~string](dst *T, b []byte, version byte) error {
	text, err := parseVersion(string(b), version)
	if err != nil {
		return err
	}
	*dst = T(text)
	return nil
}

// unmarshalJSON decodes a JSON string UUID of the given version. JSON null
// clears the receiver to its zero value; any other non-string JSON value or
// an invalid/empty string is rejected. The receiver is only mutated on success
// (or on null).
func unmarshalJSON[T ~string](dst *T, b []byte, version byte) error {
	if string(b) == "null" {
		*dst = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return ident.Wrap("uuid", err)
	}
	return unmarshalText(dst, []byte(s), version)
}

// scan reads a database/sql source value into dst. nil and empty strings
// clear the receiver to its zero value; otherwise the source must be a valid
// UUID of the given version.
func scan[T ~string](dst *T, src any, version byte) error {
	s, err := ident.ScanText(src)
	if err != nil {
		return err
	}
	if s == "" {
		*dst = ""
		return nil
	}
	return unmarshalText(dst, []byte(s), version)
}

// value converts id into a database/sql driver value. The zero value maps to
// NULL; a valid id maps to its canonical string; an invalid non-zero id is an
// error.
func value[T ~string](id T, version byte) (driver.Value, error) {
	if id == "" {
		return nil, nil
	}
	if _, err := parseVersion(string(id), version); err != nil {
		return nil, err
	}
	return string(id), nil
}

// The following methods are thin, version-specific wrappers over the generic
// helpers above. Each type carries its RFC version so that validation is
// exact: a V4 value will not accept a V1 string.

func (id V1[T]) String() string                { return string(id) }
func (id V1[T]) Valid() bool                   { return ValidV1(string(id)) }
func (id V1[T]) MarshalText() ([]byte, error)  { return marshalText(id) }
func (id *V1[T]) UnmarshalText(b []byte) error { return unmarshalText(id, b, 1) }
func (id V1[T]) MarshalJSON() ([]byte, error)  { return ident.MarshalJSON(string(id)) }
func (id *V1[T]) UnmarshalJSON(b []byte) error { return unmarshalJSON(id, b, 1) }
func (id *V1[T]) Scan(src any) error           { return scan(id, src, 1) }
func (id V1[T]) Value() (driver.Value, error)  { return value(id, 1) }

func (id V2[T]) String() string                { return string(id) }
func (id V2[T]) Valid() bool                   { return ValidV2(string(id)) }
func (id V2[T]) MarshalText() ([]byte, error)  { return marshalText(id) }
func (id *V2[T]) UnmarshalText(b []byte) error { return unmarshalText(id, b, 2) }
func (id V2[T]) MarshalJSON() ([]byte, error)  { return ident.MarshalJSON(string(id)) }
func (id *V2[T]) UnmarshalJSON(b []byte) error { return unmarshalJSON(id, b, 2) }
func (id *V2[T]) Scan(src any) error           { return scan(id, src, 2) }
func (id V2[T]) Value() (driver.Value, error)  { return value(id, 2) }

func (id V3[T]) String() string                { return string(id) }
func (id V3[T]) Valid() bool                   { return ValidV3(string(id)) }
func (id V3[T]) MarshalText() ([]byte, error)  { return marshalText(id) }
func (id *V3[T]) UnmarshalText(b []byte) error { return unmarshalText(id, b, 3) }
func (id V3[T]) MarshalJSON() ([]byte, error)  { return ident.MarshalJSON(string(id)) }
func (id *V3[T]) UnmarshalJSON(b []byte) error { return unmarshalJSON(id, b, 3) }
func (id *V3[T]) Scan(src any) error           { return scan(id, src, 3) }
func (id V3[T]) Value() (driver.Value, error)  { return value(id, 3) }

func (id V4[T]) String() string                { return string(id) }
func (id V4[T]) Valid() bool                   { return ValidV4(string(id)) }
func (id V4[T]) MarshalText() ([]byte, error)  { return marshalText(id) }
func (id *V4[T]) UnmarshalText(b []byte) error { return unmarshalText(id, b, 4) }
func (id V4[T]) MarshalJSON() ([]byte, error)  { return ident.MarshalJSON(string(id)) }
func (id *V4[T]) UnmarshalJSON(b []byte) error { return unmarshalJSON(id, b, 4) }
func (id *V4[T]) Scan(src any) error           { return scan(id, src, 4) }
func (id V4[T]) Value() (driver.Value, error)  { return value(id, 4) }

func (id V5[T]) String() string                { return string(id) }
func (id V5[T]) Valid() bool                   { return ValidV5(string(id)) }
func (id V5[T]) MarshalText() ([]byte, error)  { return marshalText(id) }
func (id *V5[T]) UnmarshalText(b []byte) error { return unmarshalText(id, b, 5) }
func (id V5[T]) MarshalJSON() ([]byte, error)  { return ident.MarshalJSON(string(id)) }
func (id *V5[T]) UnmarshalJSON(b []byte) error { return unmarshalJSON(id, b, 5) }
func (id *V5[T]) Scan(src any) error           { return scan(id, src, 5) }
func (id V5[T]) Value() (driver.Value, error)  { return value(id, 5) }

func (id V6[T]) String() string                { return string(id) }
func (id V6[T]) Valid() bool                   { return ValidV6(string(id)) }
func (id V6[T]) MarshalText() ([]byte, error)  { return marshalText(id) }
func (id *V6[T]) UnmarshalText(b []byte) error { return unmarshalText(id, b, 6) }
func (id V6[T]) MarshalJSON() ([]byte, error)  { return ident.MarshalJSON(string(id)) }
func (id *V6[T]) UnmarshalJSON(b []byte) error { return unmarshalJSON(id, b, 6) }
func (id *V6[T]) Scan(src any) error           { return scan(id, src, 6) }
func (id V6[T]) Value() (driver.Value, error)  { return value(id, 6) }

func (id V7[T]) String() string                { return string(id) }
func (id V7[T]) Valid() bool                   { return ValidV7(string(id)) }
func (id V7[T]) MarshalText() ([]byte, error)  { return marshalText(id) }
func (id *V7[T]) UnmarshalText(b []byte) error { return unmarshalText(id, b, 7) }
func (id V7[T]) MarshalJSON() ([]byte, error)  { return ident.MarshalJSON(string(id)) }
func (id *V7[T]) UnmarshalJSON(b []byte) error { return unmarshalJSON(id, b, 7) }
func (id *V7[T]) Scan(src any) error           { return scan(id, src, 7) }
func (id V7[T]) Value() (driver.Value, error)  { return value(id, 7) }

func (id V8[T]) String() string                { return string(id) }
func (id V8[T]) Valid() bool                   { return ValidV8(string(id)) }
func (id V8[T]) MarshalText() ([]byte, error)  { return marshalText(id) }
func (id *V8[T]) UnmarshalText(b []byte) error { return unmarshalText(id, b, 8) }
func (id V8[T]) MarshalJSON() ([]byte, error)  { return ident.MarshalJSON(string(id)) }
func (id *V8[T]) UnmarshalJSON(b []byte) error { return unmarshalJSON(id, b, 8) }
func (id *V8[T]) Scan(src any) error           { return scan(id, src, 8) }
func (id V8[T]) Value() (driver.Value, error)  { return value(id, 8) }
