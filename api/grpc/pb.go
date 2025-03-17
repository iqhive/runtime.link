// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package protowire parses and formats the raw wire encoding.
// See https://protobuf.dev/programming-guides/encoding.
//
// For marshaling and unmarshaling entire protobuf messages,
// use the [google.golang.org/protobuf/proto] package instead.
package grpc

import (
	"io"
	"math"
	"math/bits"
	"unsafe"

	"errors"
)

// wireNumber represents the field number.
type wireNumber int32

const (
	MinValidNumber        wireNumber = 1
	FirstReservedNumber   wireNumber = 19000
	LastReservedNumber    wireNumber = 19999
	MaxValidNumber        wireNumber = 1<<29 - 1
	DefaultRecursionLimit            = 10000
)

// IsValid reports whether the field number is semantically valid.
func (n wireNumber) IsValid() bool {
	return MinValidNumber <= n && n <= MaxValidNumber
}

// wireType represents the wire type.
type wireType int8

const (
	typeVarint     wireType = 0
	typeFixed32    wireType = 5
	typeFixed64    wireType = 1
	typeBytes      wireType = 2
	typeStartGroup wireType = 3
	typeEndGroup   wireType = 4
)

const (
	_ = -iota
	errCodeTruncated
	errCodeFieldNumber
	errCodeOverflow
	errCodeReserved
	errCodeEndGroup
	errCodeRecursionDepth
)

var (
	errFieldNumber = errors.New("invalid field number")
	errOverflow    = errors.New("variable length integer overflow")
	errReserved    = errors.New("cannot parse reserved wire type")
	errEndGroup    = errors.New("mismatching end group marker")
	errParse       = errors.New("parse error")
)

// parseError converts an error code into an error value.
// This returns nil if n is a non-negative number.
func parseError(n int) error {
	if n >= 0 {
		return nil
	}
	switch n {
	case errCodeTruncated:
		return io.ErrUnexpectedEOF
	case errCodeFieldNumber:
		return errFieldNumber
	case errCodeOverflow:
		return errOverflow
	case errCodeReserved:
		return errReserved
	case errCodeEndGroup:
		return errEndGroup
	default:
		return errParse
	}
}

// consumeField parses an entire field record (both tag and value) and returns
// the field number, the wire type, and the total length.
// This returns a negative length upon an error (see [parseError]).
//
// The total length includes the tag header and the end group marker (if the
// field is a group).
func consumeField(b []byte) (wireNumber, wireType, int) {
	num, typ, n := consumeTag(b)
	if n < 0 {
		return 0, 0, n // forward error code
	}
	m := consumeFieldValue(num, typ, b[n:])
	if m < 0 {
		return 0, 0, m // forward error code
	}
	return num, typ, n + m
}

// consumeFieldValue parses a field value and returns its length.
// This assumes that the field [wireNumber] and wire [wireType] have already been parsed.
// This returns a negative length upon an error (see [parseError]).
//
// When parsing a group, the length includes the end group marker and
// the end group is verified to match the starting field number.
func consumeFieldValue(num wireNumber, typ wireType, b []byte) (n int) {
	return consumeFieldValueD(num, typ, b, DefaultRecursionLimit)
}

func consumeFieldValueD(num wireNumber, typ wireType, b []byte, depth int) (n int) {
	switch typ {
	case typeVarint:
		_, n = consumeVarint(b)
		return n
	case typeFixed32:
		_, n = consumeFixed32(b)
		return n
	case typeFixed64:
		_, n = consumeFixed64(b)
		return n
	case typeBytes:
		_, n = consumeBytes(b)
		return n
	case typeStartGroup:
		if depth < 0 {
			return errCodeRecursionDepth
		}
		n0 := len(b)
		for {
			num2, typ2, n := consumeTag(b)
			if n < 0 {
				return n // forward error code
			}
			b = b[n:]
			if typ2 == typeEndGroup {
				if num != num2 {
					return errCodeEndGroup
				}
				return n0 - len(b)
			}

			n = consumeFieldValueD(num2, typ2, b, depth-1)
			if n < 0 {
				return n // forward error code
			}
			b = b[n:]
		}
	case typeEndGroup:
		return errCodeEndGroup
	default:
		return errCodeReserved
	}
}

// appendTag encodes num and typ as a varint-encoded tag and appends it to b.
func appendTag(b []byte, num wireNumber, typ wireType) []byte {
	return appendVarint(b, encodeTag(num, typ))
}

// consumeTag parses b as a varint-encoded tag, reporting its length.
// This returns a negative length upon an error (see [parseError]).
func consumeTag(b []byte) (wireNumber, wireType, int) {
	v, n := consumeVarint(b)
	if n < 0 {
		return 0, 0, n // forward error code
	}
	num, typ := decodeTag(v)
	if num < MinValidNumber {
		return 0, 0, errCodeFieldNumber
	}
	return num, typ, n
}

func sizeTag(num wireNumber) int {
	return sizeVarint(encodeTag(num, 0)) // wire type has no effect on size
}

// appendVarint appends v to b as a varint-encoded uint64.
func appendVarint(b []byte, v uint64) []byte {
	switch {
	case v < 1<<7:
		b = append(b, byte(v))
	case v < 1<<14:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte(v>>7))
	case v < 1<<21:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte(v>>14))
	case v < 1<<28:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte(v>>21))
	case v < 1<<35:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte(v>>28))
	case v < 1<<42:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte(v>>35))
	case v < 1<<49:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte((v>>35)&0x7f|0x80),
			byte(v>>42))
	case v < 1<<56:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte((v>>35)&0x7f|0x80),
			byte((v>>42)&0x7f|0x80),
			byte(v>>49))
	case v < 1<<63:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte((v>>35)&0x7f|0x80),
			byte((v>>42)&0x7f|0x80),
			byte((v>>49)&0x7f|0x80),
			byte(v>>56))
	default:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte((v>>35)&0x7f|0x80),
			byte((v>>42)&0x7f|0x80),
			byte((v>>49)&0x7f|0x80),
			byte((v>>56)&0x7f|0x80),
			1)
	}
	return b
}

// consumeVarint parses b as a varint-encoded uint64, reporting its length.
// This returns a negative length upon an error (see [parseError]).
func consumeVarint(b []byte) (v uint64, n int) {
	var y uint64
	if len(b) <= 0 {
		return 0, errCodeTruncated
	}
	v = uint64(b[0])
	if v < 0x80 {
		return v, 1
	}
	v -= 0x80

	if len(b) <= 1 {
		return 0, errCodeTruncated
	}
	y = uint64(b[1])
	v += y << 7
	if y < 0x80 {
		return v, 2
	}
	v -= 0x80 << 7

	if len(b) <= 2 {
		return 0, errCodeTruncated
	}
	y = uint64(b[2])
	v += y << 14
	if y < 0x80 {
		return v, 3
	}
	v -= 0x80 << 14

	if len(b) <= 3 {
		return 0, errCodeTruncated
	}
	y = uint64(b[3])
	v += y << 21
	if y < 0x80 {
		return v, 4
	}
	v -= 0x80 << 21

	if len(b) <= 4 {
		return 0, errCodeTruncated
	}
	y = uint64(b[4])
	v += y << 28
	if y < 0x80 {
		return v, 5
	}
	v -= 0x80 << 28

	if len(b) <= 5 {
		return 0, errCodeTruncated
	}
	y = uint64(b[5])
	v += y << 35
	if y < 0x80 {
		return v, 6
	}
	v -= 0x80 << 35

	if len(b) <= 6 {
		return 0, errCodeTruncated
	}
	y = uint64(b[6])
	v += y << 42
	if y < 0x80 {
		return v, 7
	}
	v -= 0x80 << 42

	if len(b) <= 7 {
		return 0, errCodeTruncated
	}
	y = uint64(b[7])
	v += y << 49
	if y < 0x80 {
		return v, 8
	}
	v -= 0x80 << 49

	if len(b) <= 8 {
		return 0, errCodeTruncated
	}
	y = uint64(b[8])
	v += y << 56
	if y < 0x80 {
		return v, 9
	}
	v -= 0x80 << 56

	if len(b) <= 9 {
		return 0, errCodeTruncated
	}
	y = uint64(b[9])
	v += y << 63
	if y < 2 {
		return v, 10
	}
	return 0, errCodeOverflow
}

// sizeVarint returns the encoded size of a varint.
// The size is guaranteed to be within 1 and 10, inclusive.
func sizeVarint(v uint64) int {
	// This computes 1 + (bits.Len64(v)-1)/7.
	// 9/64 is a good enough approximation of 1/7
	return int(9*uint32(bits.Len64(v))+64) / 64
}

// appendFixed32 appends v to b as a little-endian uint32.
func appendFixed32(b []byte, v uint32) []byte {
	return append(b,
		byte(v>>0),
		byte(v>>8),
		byte(v>>16),
		byte(v>>24))
}

// consumeFixed32 parses b as a little-endian uint32, reporting its length.
// This returns a negative length upon an error (see [parseError]).
func consumeFixed32(b []byte) (v uint32, n int) {
	if len(b) < 4 {
		return 0, errCodeTruncated
	}
	v = uint32(b[0])<<0 | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
	return v, 4
}

// sizeFixed32 returns the encoded size of a fixed32; which is always 4.
func sizeFixed32() int {
	return 4
}

// appendFixed64 appends v to b as a little-endian uint64.
func appendFixed64(b []byte, v uint64) []byte {
	return append(b,
		byte(v>>0),
		byte(v>>8),
		byte(v>>16),
		byte(v>>24),
		byte(v>>32),
		byte(v>>40),
		byte(v>>48),
		byte(v>>56))
}

// consumeFixed64 parses b as a little-endian uint64, reporting its length.
// This returns a negative length upon an error (see [parseError]).
func consumeFixed64(b []byte) (v uint64, n int) {
	if len(b) < 8 {
		return 0, errCodeTruncated
	}
	v = uint64(b[0])<<0 | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
	return v, 8
}

// sizeFixed64 returns the encoded size of a fixed64; which is always 8.
func sizeFixed64() int {
	return 8
}

// appendBytes appends v to b as a length-prefixed bytes value.
func appendBytes(b []byte, v []byte) []byte {
	return append(appendVarint(b, uint64(len(v))), v...)
}

// consumeBytes parses b as a length-prefixed bytes value, reporting its length.
// This returns a negative length upon an error (see [parseError]).
func consumeBytes(b []byte) (v []byte, n int) {
	m, n := consumeVarint(b)
	if n < 0 {
		return nil, n // forward error code
	}
	if m > uint64(len(b[n:])) {
		return nil, errCodeTruncated
	}
	return b[n:][:m], n + int(m)
}

// sizeBytes returns the encoded size of a length-prefixed bytes value,
// given only the length.
func sizeBytes(n int) int {
	return sizeVarint(uint64(n)) + n
}

// appendString appends v to b as a length-prefixed bytes value.
func appendString(b []byte, v string) []byte {
	return append(appendVarint(b, uint64(len(v))), v...)
}

// consumeString parses b as a length-prefixed bytes value, reporting its length.
// This returns a negative length upon an error (see [parseError]).
func consumeString(b []byte) (v string, n int) {
	bb, n := consumeBytes(b)
	return string(bb), n
}

// appendGroup appends v to b as group value, with a trailing end group marker.
// The value v must not contain the end marker.
func appendGroup(b []byte, num wireNumber, v []byte) []byte {
	return appendVarint(append(b, v...), encodeTag(num, typeEndGroup))
}

// consumeGroup parses b as a group value until the trailing end group marker,
// and verifies that the end marker matches the provided num. The value v
// does not contain the end marker, while the length does contain the end marker.
// This returns a negative length upon an error (see [parseError]).
func consumeGroup(num wireNumber, b []byte) (v []byte, n int) {
	n = consumeFieldValue(num, typeStartGroup, b)
	if n < 0 {
		return nil, n // forward error code
	}
	b = b[:n]

	// Truncate off end group marker, but need to handle denormalized varints.
	// Assuming end marker is never 0 (which is always the case since
	// EndGroupType is non-zero), we can truncate all trailing bytes where the
	// lower 7 bits are all zero (implying that the varint is denormalized).
	for len(b) > 0 && b[len(b)-1]&0x7f == 0 {
		b = b[:len(b)-1]
	}
	b = b[:len(b)-sizeTag(num)]
	return b, n
}

// sizeGroup returns the encoded size of a group, given only the length.
func sizeGroup(num wireNumber, n int) int {
	return n + sizeTag(num)
}

// decodeTag decodes the field [wireNumber] and wire [wireType] from its unified form.
// The [wireNumber] is -1 if the decoded field number overflows int32.
// Other than overflow, this does not check for field number validity.
func decodeTag(x uint64) (wireNumber, wireType) {
	// NOTE: MessageSet allows for larger field numbers than normal.
	if x>>3 > uint64(math.MaxInt32) {
		return -1, 0
	}
	return wireNumber(x >> 3), wireType(x & 7)
}

// encodeTag encodes the field [wireNumber] and wire [wireType] into its unified form.
func encodeTag(num wireNumber, typ wireType) uint64 {
	return uint64(num)<<3 | uint64(typ&7)
}

// decodeZigZag decodes a zig-zag-encoded uint64 as an int64.
//
//	Input:  {…,  5,  3,  1,  0,  2,  4,  6, …}
//	Output: {…, -3, -2, -1,  0, +1, +2, +3, …}
func decodeZigZag(x uint64) int64 {
	return int64(x>>1) ^ int64(x)<<63>>63
}

// encodeZigZag encodes an int64 as a zig-zag-encoded uint64.
//
//	Input:  {…, -3, -2, -1,  0, +1, +2, +3, …}
//	Output: {…,  5,  3,  1,  0,  2,  4,  6, …}
func encodeZigZag(x int64) uint64 {
	return uint64(x<<1) ^ uint64(x>>63)
}

// decodeBool decodes a uint64 as a bool.
//
//	Input:  {    0,    1,    2, …}
//	Output: {false, true, true, …}
func decodeBool(x uint64) bool {
	return x != 0
}

// encodeBool encodes a bool as a uint64.
//
//	Input:  {false, true}
//	Output: {    0,    1}
func encodeBool(x bool) uint64 {
	if x {
		return 1
	}
	return 0
}

func encodeInt(x int64) uint64 {
	return *(*uint64)(unsafe.Pointer(&x))
}
