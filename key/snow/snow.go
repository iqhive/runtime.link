// Package snow provides a production-ready Snowflake ID type.
//
// A Snowflake ID (https://en.wikipedia.org/wiki/Snowflake_ID) is a 64-bit
// integer that embeds a timestamp, a worker identifier, and a per-millisecond
// sequence, so ids are time-ordered.
//
// # Uniqueness
//
// A running process never repeats an id. Between processes, uniqueness rests
// on the worker identifier:
//
//   - New always uses worker identifier 0, so its ids are unique within one
//     process only. Two processes calling New in the same millisecond can
//     produce identical ids.
//
//   - NewWith takes the worker identifier from the caller. Ids are unique
//     across processes when the deployment gives every running process its own
//     worker identifier, which this package does not do for it. Use NewWith
//     whenever more than one process generates ids for the same table.
//
// The generator's state is held in memory, so nothing carries over a restart.
// A restarted process can repeat ids from its previous run if it reaches a
// timestamp that run already used, because the clock was set back across the
// restart or because that run was issuing ids ahead of the clock at more than
// 4096 per millisecond. The deployment must ensure the clock never restarts
// behind the last id issued.
//
// The 41-bit timestamp runs out in July 2080. NewWith then returns ErrOverflow.
// New cannot report it: its ids turn negative, and Parse refuses negative text.
//
// # Representation
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

	ident "github.com/iqhive/runtime.link/key/internal/id"
)

// epoch is the Snowflake epoch (Twitter's, 2010-11-04) in milliseconds. The
// 41-bit timestamp is measured from this point.
const epoch int64 = 1288834974657

// workerBits is the number of bits used for the worker identifier.
const workerBits = 10

// sequenceBits is the number of bits used for the per-millisecond sequence.
const sequenceBits = 12

// maxWorker is the largest worker identifier that fits in workerBits.
const maxWorker = 1<<workerBits - 1

// maxTimestamp is the largest timestamp that leaves an id non-negative.
const maxTimestamp = 1<<(63-workerBits-sequenceBits) - 1

var (
	// ErrCharacters is returned when a Snowflake string is not a decimal
	// integer.
	ErrCharacters = errors.New("snow: bad data characters")

	// ErrOverflow is returned when a Snowflake string overflows int64, or
	// when NewWith is called after the 41-bit timestamp has run out.
	ErrOverflow = errors.New("snow: overflow")

	// ErrWorker is returned by NewWith for a worker identifier outside the
	// range 0 to 1023.
	ErrWorker = errors.New("snow: worker identifier out of range")
)

// FlakeID is a Snowflake identifier for a value of type T.
type FlakeID[T any] int64

var (
	genMu     sync.Mutex
	genLastMs int64
	genSeq    int64
)

// New returns a new Snowflake identifier for the current time, using worker
// identifier 0 and a monotonically increasing per-millisecond sequence. Its
// ids are unique within one process only; see the package documentation.
//
// It is safe for concurrent use and guarantees that ids are strictly
// increasing across all callers, even if the wall clock moves backwards.
func New[T any]() FlakeID[T] {
	return FlakeID[T](generate(time.Now(), 0))
}

// NewWith returns a new Snowflake identifier for the current time using the
// given worker identifier, for deployments that assign one to each process.
// Assigning them is the only way to guarantee ids are unique across processes;
// see the package documentation.
//
// It returns ErrWorker if worker is outside the range 0 to 1023, and
// ErrOverflow once the 41-bit timestamp has run out.
//
// New and NewWith share one generator, so a process never repeats an id
// whichever it calls. Ids strictly increase for any one worker, but ids from
// different workers have no defined order within a millisecond.
func NewWith[T any](worker int64) (FlakeID[T], error) {
	v, err := generateChecked(time.Now(), worker)
	return FlakeID[T](v), err
}

// Parse parses a decimal string as a Snowflake identifier. It accepts any
// base-10 integer strconv.ParseInt accepts that is not negative, so a leading
// "+" and leading zeros are allowed, and "0" or "-0" give the zero value.
func Parse[T any](s string) (FlakeID[T], error) {
	v, err := parse(s)
	return FlakeID[T](v), err
}

// Valid reports whether s is a well-formed decimal Snowflake, by the same rules
// as Parse.
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

// generateChecked is generate for callers that can report an id it could not
// represent.
func generateChecked(t time.Time, worker int64) (int64, error) {
	if worker < 0 || worker > maxWorker {
		return 0, ident.Wrap("snow", ErrWorker)
	}
	if t.UnixMilli()-epoch > maxTimestamp {
		return 0, ident.Wrap("snow", ErrOverflow)
	}
	return generate(t, worker), nil
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

// Valid reports whether id is set, which is any value but zero. A negative id
// is valid here although Parse refuses its text, because ids decoded from JSON
// numbers and database integers are not checked for sign.
func (id FlakeID[T]) Valid() bool { return id != 0 }

// MarshalText renders the id as its decimal text, rejecting the zero value.
// A negative id is written with its sign.
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

// MarshalJSON renders the id as a JSON number, rejecting the zero value. A
// negative id is written with its sign.
//
// It cannot see a `json:",string"` tag, so such a field is still written as a
// number. The decoder in encoding/json before Go 1.27 refuses that number
// before UnmarshalJSON is called, so the tag does not round trip on those
// toolchains.
func (id FlakeID[T]) MarshalJSON() ([]byte, error) {
	if id == 0 {
		return nil, ident.Wrap("snow", ident.ErrInvalid)
	}
	return []byte(id.String()), nil
}

// UnmarshalJSON decodes a JSON number into the id: any number that fits an
// int64, including zero and negatives. JSON null clears the receiver to its
// zero value. A JSON string holding the decimal text String gives for a
// positive id is accepted too, because that is how encoding/json passes a map
// key here before Go 1.27 and a `json:",string"` field from Go 1.27. Anything
// else fails with the error decoding it into an int64 gives. The receiver is
// only mutated on success (or on null).
func (id *FlakeID[T]) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*id = 0
		return nil
	}
	var v int64
	err := json.Unmarshal(b, &v)
	if err == nil {
		*id = FlakeID[T](v)
		return nil
	}
	if q, ok := quotedID(b); ok {
		*id = FlakeID[T](q)
		return nil
	}
	return ident.Wrap("snow", err)
}

// quotedID reads b as a JSON string holding the text String gives for a
// positive id. Other text is refused so that the quoted form adds no spelling
// of an id beyond the one this package writes.
func quotedID(b []byte) (int64, bool) {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	return v, err == nil && isCanonical(v, s)
}

// isCanonical reports whether v is a positive id and s is how it prints.
func isCanonical(v int64, s string) bool {
	return v > 0 && strconv.FormatInt(v, 10) == s
}

// Scan reads a database/sql source value into the id. It accepts nil (which
// clears the receiver), any int64, and string or []byte text that Parse
// accepts. Empty text clears the receiver as NULL does, since some drivers
// return an empty value for NULL.
func (id *FlakeID[T]) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*id = 0
		return nil
	case int64:
		*id = FlakeID[T](v)
		return nil
	case string:
		return id.scanText(v)
	case []byte:
		return id.scanText(string(v))
	default:
		return ident.Wrap("snow", errors.New("cannot scan into a snowflake"))
	}
}

// scanText applies Scan's rules to text.
func (id *FlakeID[T]) scanText(s string) error {
	if s == "" {
		*id = 0
		return nil
	}
	return id.UnmarshalText([]byte(s))
}

// Value converts the id into a database/sql driver value. The zero value maps
// to NULL; otherwise the id, negative or not, is returned as an int64
// (BIGINT), which is the natural storage form for a numeric primary key.
func (id FlakeID[T]) Value() (driver.Value, error) {
	if id == 0 {
		return nil, nil
	}
	return int64(id), nil
}
