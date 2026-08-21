// Package snow provides a production-ready Snowflake ID type.
//
// A Snowflake ID (https://en.wikipedia.org/wiki/Snowflake_ID) is a 64-bit
// integer that embeds a timestamp, a worker identifier, and a per-millisecond
// sequence, so ids are time-ordered and unique across a cluster without
// coordination.
//
// Unlike the other identifier packages, FlakeID is int64-backed rather than
// string-backed. This is intentional: a snowflake is fundamentally a numeric
// primary key, so it marshals as a JSON number and is stored as a BIGINT in
// SQL. The text form is its decimal representation.
package snow

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"time"

	ident "runtime.link/key/internal/id"
)

// epoch is the Snowflake epoch (Twitter's, 2010-11-04) in milliseconds. The
// 41-bit timestamp is measured from this point.
const epoch int64 = 1288834974657

// workerBits is the number of bits used for the worker identifier.
const workerBits = 10

// sequenceBits is the number of bits used for the per-millisecond sequence.
const sequenceBits = 12

var (
	// ErrCharacters is returned when a Snowflake string is not a decimal
	// integer.
	ErrCharacters = errors.New("snow: bad data characters")

	// ErrOverflow is returned when a Snowflake string overflows int64.
	ErrOverflow = errors.New("snow: overflow")
)

// FlakeID is a Snowflake identifier for a value of type T.
type FlakeID[T any] int64

var (
	genMu     sync.Mutex
	genLastMs int64
	genSeq    int64
)

// New returns a new Snowflake identifier for the current time, using worker
// identifier 0 and a monotonically increasing per-millisecond sequence.
//
// It is safe for concurrent use and guarantees that ids are strictly
// increasing across all callers, even if the wall clock moves backwards.
func New[T any]() FlakeID[T] {
	return FlakeID[T](generate(time.Now(), 0))
}

// Parse parses a decimal string as a Snowflake identifier.
func Parse[T any](s string) (FlakeID[T], error) {
	v, err := parse(s)
	return FlakeID[T](v), err
}

// Valid reports whether s is a well-formed decimal Snowflake.
func Valid(s string) bool { _, err := parse(s); return err == nil }

// generate produces a Snowflake id for the given time and worker, guaranteeing
// strict monotonicity even under clock rollback.
func generate(t time.Time, worker int64) int64 {
	ms := t.UnixMilli() - epoch

	genMu.Lock()
	if ms < genLastMs {
		// The wall clock moved backwards. Clamp to the last timestamp so ids
		// remain increasing rather than going backwards.
		ms = genLastMs
	}
	if ms == genLastMs {
		genSeq = (genSeq + 1) & ((1 << sequenceBits) - 1)
		if genSeq == 0 {
			// Sequence exhausted within this millisecond: advance to the next.
			ms++
		}
	} else {
		genSeq = 0
	}
	genLastMs = ms
	seq := genSeq
	genMu.Unlock()

	return (ms << (workerBits + sequenceBits)) | (worker << sequenceBits) | seq
}

// parse validates a decimal string and returns the int64 value. Snowflakes
// are non-negative, so negative values are rejected.
func parse(s string) (int64, error) {
	if s == "" {
		return 0, ident.Wrap("snow", ident.ErrInvalid)
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return 0, ident.Wrap("snow", ErrOverflow)
		}
		return 0, ident.Wrap("snow", ErrCharacters)
	}
	if v < 0 {
		return 0, ident.Wrap("snow", ident.ErrInvalid)
	}
	return v, nil
}

func (id FlakeID[T]) String() string { return strconv.FormatInt(int64(id), 10) }

func (id FlakeID[T]) Valid() bool { return id != 0 }

// MarshalText renders the id as its decimal text, rejecting the zero value.
func (id FlakeID[T]) MarshalText() ([]byte, error) {
	if id == 0 {
		return nil, ident.Wrap("snow", ident.ErrInvalid)
	}
	return []byte(id.String()), nil
}

// UnmarshalText parses decimal text into the id. The receiver is only mutated
// on success.
func (id *FlakeID[T]) UnmarshalText(b []byte) error {
	v, err := parse(string(b))
	if err != nil {
		return err
	}
	*id = FlakeID[T](v)
	return nil
}

// MarshalJSON renders the id as a JSON number, rejecting the zero value.
func (id FlakeID[T]) MarshalJSON() ([]byte, error) {
	if id == 0 {
		return nil, ident.Wrap("snow", ident.ErrInvalid)
	}
	return []byte(id.String()), nil
}

// UnmarshalJSON decodes a JSON number into the id. JSON null clears the
// receiver to its zero value.
func (id *FlakeID[T]) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*id = 0
		return nil
	}
	var v int64
	if err := json.Unmarshal(b, &v); err != nil {
		return ident.Wrap("snow", err)
	}
	*id = FlakeID[T](v)
	return nil
}

// Scan reads a database/sql source value into the id. It accepts int64,
// string, []byte, and nil (which clears the receiver).
func (id *FlakeID[T]) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*id = 0
		return nil
	case int64:
		*id = FlakeID[T](v)
		return nil
	case string:
		return id.UnmarshalText([]byte(v))
	case []byte:
		return id.UnmarshalText(v)
	default:
		return ident.Wrap("snow", errors.New("cannot scan into a snowflake"))
	}
}

// Value converts the id into a database/sql driver value. The zero value maps
// to NULL; otherwise the id is returned as an int64 (BIGINT), which is the
// natural storage form for a numeric primary key.
func (id FlakeID[T]) Value() (driver.Value, error) {
	if id == 0 {
		return nil, nil
	}
	return int64(id), nil
}
