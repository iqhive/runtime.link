// Package uuid provides a standard UUIDv4 reference type.
package uuid

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
)

// For is a UUIDv4 reference value.
type For[T any] [16]byte

// New returns a new UUIDv4 reference.
func New[T any]() For[T] {
	var buf [16]byte
	_, err := rand.Read(buf[:])
	if err != nil {
		panic(err)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return For[T](buf)
}

// String implements the [fmt.Stringer] interface.
func (ref For[T]) String() string {
	b, err := ref.MarshalText()
	if err != nil {
		panic(err)
	}
	return string(b)
}

// MarshalText implements the [encoding.TextMarshaler] interface.
func (ref For[T]) MarshalText() ([]byte, error) {
	var buf [36]byte
	hex.Encode(buf[:], ref[:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], ref[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], ref[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], ref[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:], ref[10:])
	return buf[:], nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (ref *For[T]) UnmarshalText(text []byte) error {
	if len(text) != 36 {
		return errors.New("invalid UUID length")
	}
	if text[8] != '-' || text[13] != '-' || text[18] != '-' || text[23] != '-' {
		return errors.New("invalid UUID format")
	}
	if _, err := hex.Decode(ref[:4], text[:8]); err != nil {
		return err
	}
	if _, err := hex.Decode(ref[4:6], text[9:13]); err != nil {
		return err
	}
	if _, err := hex.Decode(ref[6:8], text[14:18]); err != nil {
		return err
	}
	if _, err := hex.Decode(ref[8:10], text[19:23]); err != nil {
		return err
	}
	if _, err := hex.Decode(ref[10:], text[24:]); err != nil {
		return err
	}
	return nil
}
