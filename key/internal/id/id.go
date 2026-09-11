// Package id contains small, private helpers shared by the identifier
// packages under github.com/iqhive/runtime.link/key (uuid, ulid, ksuid, cuid, nanoid, snow).
//
// The identifier packages deliberately keep their own parsing/generation
// logic, but the *boundary* behavior (text/JSON marshaling, database/sql
// scanning and valuing, and error reporting) is identical across every
// format. Centralizing it here guarantees that all formats behave the same
// way at the edges, so a caller never has to remember which format treats
// NULL or an empty string differently.
//
// This package is internal and must not be imported by code outside the
// github.com/iqhive/runtime.link/key tree. It is not part of the public API.
package id

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrInvalid is the sentinel error wrapped by every identifier parse and
// validation failure. Callers can use errors.Is(err, id.ErrInvalid) to detect
// "this is not a well-formed identifier of the expected format" regardless of
// which concrete package produced the error.
//
// It is a single sentinel shared by all formats so that application code can
// treat malformed identifiers uniformly. Format-specific context is attached
// by Wrap, so the concrete error still tells the caller which format failed.
var ErrInvalid = errors.New("invalid identifier")

// Wrap attaches the given format/context label to a validation error.
//
// The label is the package or format name (for example "uuid v7" or "ksuid").
// We deliberately do not embed the offending input string: identifier values
// are frequently sensitive (they can be used to enumerate resources), and
// logging them in errors is a common source of accidental leakage. The label
// is enough for a caller to act on the error.
func Wrap(format string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", format, err)
}

// ScanText normalizes a database/sql source value into the canonical string
// form of an identifier.
//
// It accepts exactly the three source types that database drivers are
// documented to produce for text columns:
//
//   - nil       -> the empty string (a SQL NULL)
//   - string    -> used verbatim
//   - []byte    -> used verbatim (many drivers return VARCHAR/TEXT as []byte)
//
// Any other source type is an error, because silently coercing an integer or
// float into an identifier would hide a schema mismatch.
//
// The empty string is returned unchanged (not an error) so that each package
// can decide how to represent absence. The convention used by all identifier
// packages is that an empty string from the database is treated the same as
// NULL: it clears the receiver to its zero value. This matches the behavior of
// the reference ksuid implementation and avoids surprising callers whose
// databases return ” rather than NULL.
func ScanText(src any) (string, error) {
	switch v := src.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return "", fmt.Errorf("cannot scan %T into an identifier", src)
	}
}

// Value converts the canonical string form of an identifier into a
// database/sql driver value.
//
// The zero value (empty string) maps to SQL NULL (nil, nil). This is the
// "zero means absence" policy: an unset identifier is stored as NULL rather
// than as an invalid empty string. A non-empty value is returned as a string,
// which is portable across PostgreSQL, MySQL, and SQLite text/varchar columns.
func Value(s string) (driver.Value, error) {
	if s == "" {
		return nil, nil
	}
	return s, nil
}

// MarshalJSON renders an identifier as a JSON string.
//
// The zero value (empty string) is rejected rather than serialized as an empty
// string: emitting "" for an unset identifier would silently persist an
// invalid value and corrupt the data boundary. A valid identifier marshals to
// its canonical text. Because identifiers are string-backed types, this
// method is required so that encoding/json does not fall back to the default
// string encoding, which would bypass the zero-value guard.
func MarshalJSON(s string) ([]byte, error) {
	if s == "" {
		return nil, Wrap("identifier", ErrInvalid)
	}
	return json.Marshal(s)
}
