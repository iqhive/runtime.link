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
// validation failure. Code in this tree can use errors.Is(err, id.ErrInvalid)
// to detect "this is not a well-formed identifier of the expected format"
// regardless of which concrete package produced the error.
//
// It is a single sentinel shared by all formats so that application code can
// treat malformed identifiers uniformly. Format-specific context is attached
// by Wrap, so the concrete error still tells the caller which format failed.
// A failed entropy source says nothing about an identifier, so those errors
// do not match it and must not go through Wrap. Nor do the nanoid parse
// failures, which are its bare ErrSize and ErrCharacters because callers
// compare them with ==.
var ErrInvalid = errors.New("invalid identifier")

// invalid is what fmt.Errorf("%s: %w") returns, plus a match for ErrInvalid.
// Callers outside this tree cannot name ErrInvalid, so they must keep seeing
// that error: the same message, and errors.Unwrap giving the cause itself.
// That is why the match comes from an Is method and not a second Unwrap.
type invalid struct {
	msg   string
	cause error
}

func (e *invalid) Error() string        { return e.msg }
func (e *invalid) Unwrap() error        { return e.cause }
func (e *invalid) Is(target error) bool { return target == ErrInvalid }

// Wrap attaches the given format/context label to a validation error. The
// result matches both err and ErrInvalid, and errors.Unwrap returns err.
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
	return &invalid{msg: format + ": " + err.Error(), cause: err}
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
		return "", &invalid{msg: fmt.Sprintf("cannot scan %T into an identifier", src)}
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
// invalid value and corrupt the data boundary. Any other value is written
// exactly as held, without validating or canonicalizing it: ids made by Parse,
// New or the decoders are already canonical, and one built by converting a
// string keeps the text it was given. Because identifiers are string-backed
// types, this method is required so that encoding/json does not fall back to
// the default string encoding, which would bypass the zero-value guard.
func MarshalJSON(s string) ([]byte, error) {
	if s == "" {
		return nil, Wrap("identifier", ErrInvalid)
	}
	return json.Marshal(s)
}
