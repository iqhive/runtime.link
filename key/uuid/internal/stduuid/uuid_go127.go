//go:build go1.27

// Package stduuid is an internal compatibility layer that exposes a stable,
// minimal UUID API to the runtime.link/key/uuid package regardless of the Go
// toolchain version used to build it.
//
// Go 1.27 introduced a UUID implementation into the standard library
// (package "uuid", https://pkg.go.dev/uuid). We want to use that
// implementation when it is available and to fall back to a bundled copy of
// the same code on older toolchains, so that the public key/uuid package has
// exactly one code path and no third-party UUID dependency.
//
// The API exposed here mirrors the subset of the standard library's uuid
// package that key/uuid needs: Parse, New, NewV4, NewV7, and the UUID value
// type. Because UUID is an alias to the standard library type, callers can
// also use its String/MarshalText/UnmarshalText methods directly.
//
// This package is internal and must not be imported outside runtime.link/key.
package stduuid

import "uuid"

// UUID is the 16-byte RFC 9562 Universally Unique Identifier value type.
// On Go 1.27+ it is an alias for the standard library type, so key/uuid can
// interoperate directly with values produced by the standard library.
type UUID = uuid.UUID

// Parse parses a string representation of a UUID into its 16-byte form.
// It accepts the four textual forms documented by the standard library
// (dashed, compact, braced, and urn:uuid: prefixed), case-insensitively.
func Parse(s string) (UUID, error) { return uuid.Parse(s) }

// New returns a new UUID using the algorithm recommended for most purposes.
// In Go 1.27 this is equivalent to NewV4.
func New() UUID { return uuid.New() }

// NewV4 returns a new version 4 (random) UUID.
func NewV4() UUID { return uuid.NewV4() }

// NewV7 returns a new version 7 (time-ordered) UUID.
func NewV7() UUID { return uuid.NewV7() }

// Nil returns the Nil UUID (all zeros). Re-exported so the stduuid
// API is identical on both build-tag backends.
func Nil() UUID { return uuid.Nil() }

// Max returns the Max UUID (all ones).
func Max() UUID { return uuid.Max() }

// MustParse returns the UUID represented by s, panicking on error.
func MustParse(s string) UUID { return uuid.MustParse(s) }
