// Package sodium provides a specification for the SODIUM standard database interface.
package sodium

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/iqhive/runtime.link/xyz"
)

// Database that supports the SODIUM interface.
type Database interface {
	// Search the specified [Table] for [Value]s that match the given [Query].
	// Whenever a result is found, it is sent to the specified channel, which
	// is closed when the search is over, however it ended. The search will be
	// cancelled if the managed context is cancelled. A send may be non-blocking,
	// in which case ensure that the channel is buffered appropriately to avoid
	// errors, or it may wait for room in the channel, so that a reader slower
	// than the search loses no result. A send that waits stops waiting when the
	// managed context ends: that cancels the search, and is how a reader that
	// wants no more results says so. A search that is part of a longer
	// transaction cannot be cancelled without it, so there the reader takes
	// every result until the channel is closed.
	Search(Table, Query, chan<- []Value) Job
	// Output calculates the requested [Stats] for the given table and
	// returns them to the specified channel, which is closed when the
	// output is complete. The output will be cancelled if the managed
	// context is cancelled.
	Output(Table, Query, Stats, chan<- []Value) Job
	// Delete should remove any records that match the given query from
	// the table. A finite [Range] must be specified, if the [Range] is
	// empty, the operation will fail. Cannot be cancelled.
	Delete(Table, Query) Job
	// Insert a [Value] into the table. If the value already exists, the
	// flag determines whether the operation should fail (false) or overwrite
	// the existing value (true). When overwriting, a non-empty [Query] must
	// match the existing value, else it is left as-is and the operation
	// affects nothing. Cannot be cancelled. The resulting keys and values
	// will be written into the provided slices.
	Insert(Table, []Value, bool, Query, []Value) Job
	// Update should apply the given patch to each [Value]s in
	// the table that matches the given [Query]. A finite [Range]
	// must be specified, if the [Range] is empty, the operation will fail.
	//  Cannot be cancelled.
	Update(Table, Query, Patch) Job
	// Manage returns a channel that will manage the execution of the given jobs
	// within the transaction level specified by the given [Transaction]. Close
	// the channel to commit the transaction, or send a nil [Job] (or cancel the
	// context) to rollback the transaction.
	//
	// A [Database] that is a [Committer] commits differently, so that whoever
	// commits can know whether the commit was made: see [Committer].
	Manage(context.Context, Transaction) (chan<- Job, error)
}

// Committer is a [Database] that commits a transaction only when it is sent a
// [Commit], and says how the commit went. A [Database] that is not one commits
// when the channel returned by [Database.Manage] is closed, which leaves
// whoever closes it unable to know whether the commit was made, so a commit
// that fails is reported as made.
//
// For a Committer, there is no other way to commit than to send a [Commit]: a
// channel closed without it rolls the transaction back, as does a nil [Job] or
// the end of the context, so that a transaction nobody finished is never kept.
// A [Commit] or a nil [Job] is the last thing sent, and the channel is closed
// once the sender is done with it, whatever was sent.
//
// A [Commit] that is received is always answered, with [Commit.Done]: with nil
// once the transaction has been committed and not before, and otherwise with
// what kept it from being committed, the end of the context included. The
// sender waits for the answer before it reports the writes of the transaction
// as made, and after an answer of nil nothing rolls them back, the end of the
// context included.
type Committer interface {
	Database

	// AnswersCommits reports whether the [Database] commits as a Committer
	// does. If it reports false, the channel returned by [Database.Manage] is
	// closed to commit, as for any other [Database].
	AnswersCommits() bool
}

// AnswersCommits reports whether the [Database] is a [Committer] that answers
// each [Commit] it is sent, rather than one that commits when its channel is
// closed.
func AnswersCommits(db Database) bool {
	committer, ok := db.(Committer)
	return ok && committer.AnswersCommits()
}

// Commit is the [Job] that asks for the transaction it is sent to to be
// committed, see [Committer]. Only a [Committer] is sent one. It is a request rather than work, so a
// [Database] acts on it instead of running it, and says how that went with
// [Commit.Done]. Each transaction is sent a [Commit] of its own, made with
// [NewCommit].
type Commit struct {
	answer   sync.Once
	answered chan struct{} // closed once err is set.
	err      error
}

// NewCommit returns the [Commit] to send to a transaction.
func NewCommit() *Commit {
	return &Commit{answered: make(chan struct{})}
}

// Done is called by the [Database] to say how the commit went: with nil once
// the transaction has been committed, and otherwise with the reason it was
// not. Only the first call counts.
func (commit *Commit) Done(err error) {
	commit.answer.Do(func() {
		commit.err = err
		close(commit.answered)
	})
}

// Wait for the [Database] to say how the commit went. An answer that is there
// is returned even if the context has ended, as a commit that was made stays
// made. If the context ends first its error is returned, and whether the
// transaction was committed is then not known.
func (commit *Commit) Wait(ctx context.Context) (int, error) {
	select {
	case <-commit.answered:
		return 0, commit.err
	case <-ctx.Done():
	}
	select {
	case <-commit.answered:
		return 0, commit.err
	default:
		return 0, ctx.Err()
	}
}

type Value xyz.Tagged[any, struct {
	Bool    xyz.Case[Value, bool]      `json:"bool"`
	Int8    xyz.Case[Value, int8]      `json:"int8"`
	Int16   xyz.Case[Value, int16]     `json:"int16"`
	Int32   xyz.Case[Value, int32]     `json:"int32"`
	Int64   xyz.Case[Value, int64]     `json:"int64"`
	Uint8   xyz.Case[Value, uint8]     `json:"uint8"`
	Uint16  xyz.Case[Value, uint16]    `json:"uint16"`
	Uint32  xyz.Case[Value, uint32]    `json:"uint32"`
	Uint64  xyz.Case[Value, uint64]    `json:"uint64"`
	Float32 xyz.Case[Value, float32]   `json:"float32"`
	Float64 xyz.Case[Value, float64]   `json:"float64"`
	String  xyz.Case[Value, string]    `json:"string"`
	Bytes   xyz.Case[Value, []byte]    `json:"bytes"`
	Time    xyz.Case[Value, time.Time] `json:"time"`
}]

var Values = xyz.AccessorFor(Value.Values)

func (val Value) String() string {
	switch xyz.ValueOf(val) {
	case Values.Bool:
		return strconv.FormatBool(Values.Bool.Get(val))
	case Values.Int8:
		return strconv.FormatInt(int64(Values.Int8.Get(val)), 10)
	case Values.Int16:
		return strconv.FormatInt(int64(Values.Int16.Get(val)), 10)
	case Values.Int32:
		return strconv.FormatInt(int64(Values.Int32.Get(val)), 10)
	case Values.Int64:
		return strconv.FormatInt(Values.Int64.Get(val), 10)
	case Values.Uint8:
		return strconv.FormatUint(uint64(Values.Uint8.Get(val)), 10)
	case Values.Uint16:
		return strconv.FormatUint(uint64(Values.Uint16.Get(val)), 10)
	case Values.Uint32:
		return strconv.FormatUint(uint64(Values.Uint32.Get(val)), 10)
	case Values.Uint64:
		return strconv.FormatUint(Values.Uint64.Get(val), 10)
	case Values.Float32:
		return strconv.FormatFloat(float64(Values.Float32.Get(val)), 'f', -1, 32)
	case Values.Float64:
		return strconv.FormatFloat(Values.Float64.Get(val), 'f', -1, 64)
	case Values.String:
		return Values.String.Get(val)
	case Values.Bytes:
		return base64.StdEncoding.EncodeToString(Values.Bytes.Get(val))
	case Values.Time:
		return Values.Time.Get(val).Format(time.RFC3339)
	default:
		return ""
	}
}

func (val Value) IsZero() bool {
	switch xyz.ValueOf(val) {
	case Values.Bool:
		return Values.Bool.Get(val) == false
	case Values.Int8:
		return Values.Int8.Get(val) == 0
	case Values.Int16:
		return Values.Int16.Get(val) == 0
	case Values.Int32:
		return Values.Int32.Get(val) == 0
	case Values.Int64:
		return Values.Int64.Get(val) == 0
	case Values.Uint8:
		return Values.Uint8.Get(val) == 0
	case Values.Uint16:
		return Values.Uint16.Get(val) == 0
	case Values.Uint32:
		return Values.Uint32.Get(val) == 0
	case Values.Uint64:
		return Values.Uint64.Get(val) == 0
	case Values.Float32:
		return Values.Float32.Get(val) == 0
	case Values.Float64:
		return Values.Float64.Get(val) == 0
	case Values.String:
		return Values.String.Get(val) == ""
	case Values.Bytes:
		return len(Values.Bytes.Get(val)) == 0
	case Values.Time:
		return Values.Time.Get(val).IsZero()
	default:
		return false
	}
}

// Callback should be called for each result within a search.
type Callback func() error

// Transaction flags.
type Transaction uint64

const (
	DirtyReads Transaction = 1 << iota // means the transaction can read uncommitted data.
	ReadWrites                         // means the transaction can read after it writes.
	LockWrites                         // means that all writes will be locked until the transaction ends.
	GlobalLock                         // means that the database will be locked until the transaction ends.
)

// Table represents a distinct collection of structured data
// within a database.
type Table struct {
	_ struct{}

	Name string

	Index []Column
	Value []Column

	Joins []Join
}

// Range is an half open range (or slice) on the data
// to be operated on. Index starts at 0. Range{0,1}
// means the first element, Range{0,2} means the first
// two elements, etc.
type Range struct {
	From int
	Upto int
}

type Query []Expression

// Join relationship.
type Join struct {
	_ struct{}

	On    xyz.Pair[Column, Column]
	Table Table
}

type Column struct {
	_ struct{}

	Name string
	Type xyz.TypeOf[Value]
	Tags reflect.StructTag
}

// columnJSON is a [Column] as JSON. The type of a column is one of the cases
// of [Value], which has no fields to write, so it is written as the name that
// case has in the JSON of a [Value].
type columnJSON struct {
	Name string
	Type string
	Tags reflect.StructTag
}

// MarshalJSON implements [json.Marshaler].
func (column Column) MarshalJSON() ([]byte, error) {
	written := columnJSON{Name: column.Name, Tags: column.Tags}
	if column.Type != nil {
		name, err := column.Type.Key()
		if err != nil {
			return nil, err
		}
		written.Type = name
	}
	return json.Marshal(written)
}

// UnmarshalJSON implements [json.Unmarshaler]. A type that is not the name of
// a case of [Value] is an error, rather than a column of no type.
func (column *Column) UnmarshalJSON(data []byte) error {
	var read columnJSON
	if err := json.Unmarshal(data, &read); err != nil {
		return err
	}
	decoded := Column{Name: read.Name, Tags: read.Tags}
	if read.Type != "" {
		kind, ok := typeNamed(read.Type)
		if !ok {
			return fmt.Errorf("sodium: unknown column type %q", read.Type)
		}
		decoded.Type = kind
	}
	*column = decoded
	return nil
}

// typeNamed returns the case of [Value] that has the given name in JSON.
func typeNamed(name string) (xyz.TypeOf[Value], bool) {
	cases := reflect.ValueOf(Values)
	for i := range cases.NumField() {
		kind, ok := cases.Field(i).Interface().(xyz.TypeOf[Value])
		if !ok {
			continue
		}
		if key, err := kind.Key(); err == nil && key == name {
			return kind, true
		}
	}
	return nil, false
}

// Job of SQL job.
type Job interface {
	Wait(context.Context) (int, error)
}

// WhereExpression within a [Query].
type WhereExpression xyz.Tagged[any, struct {
	Min xyz.Case[WhereExpression, xyz.Pair[Column, Value]] `json:"min"`
	Max xyz.Case[WhereExpression, xyz.Pair[Column, Value]] `json:"max"`

	MoreThan xyz.Case[WhereExpression, xyz.Pair[Column, Value]] `json:"more_than"`
	LessThan xyz.Case[WhereExpression, xyz.Pair[Column, Value]] `json:"less_than"`
}]

var WhereExpressions = xyz.AccessorFor(WhereExpression.Values)

// MatchExpression within a [Query].
type MatchExpression xyz.Tagged[any, struct {
	Contains  xyz.Case[MatchExpression, xyz.Pair[Column, string]] `json:"contains"`
	HasPrefix xyz.Case[MatchExpression, xyz.Pair[Column, string]] `json:"has_prefix"`
	HasSuffix xyz.Case[MatchExpression, xyz.Pair[Column, string]] `json:"has_suffix"`
}]

var MatchExpressions = xyz.AccessorFor(MatchExpression.Values)

// OrderExpression within a [Query]. A [Query] with more than one is ordered
// by them in the order they are written: by the first, then, where that one
// leaves results equal, by the second, and so on, as ORDER BY does in SQL.
type OrderExpression xyz.Tagged[any, struct {
	Increasing xyz.Case[OrderExpression, Column] `json:"increasing"`
	Decreasing xyz.Case[OrderExpression, Column] `json:"decreasing"`
}]

var OrderExpressions = xyz.AccessorFor(OrderExpression.Values)

// Expression within a [Query].
type Expression xyz.Tagged[any, struct {
	Value xyz.Case[Expression, bool]                    `json:"value"`
	Index xyz.Case[Expression, xyz.Pair[Column, Value]] `json:"index"`
	Where xyz.Case[Expression, WhereExpression]         `json:"where"`
	Match xyz.Case[Expression, MatchExpression]         `json:"match"`
	Order xyz.Case[Expression, OrderExpression]         `json:"order"`
	Range xyz.Case[Expression, Range]                   `json:"range"`
	Empty xyz.Case[Expression, Column]                  `json:"empty"`
	Avoid xyz.Case[Expression, Expression]              `json:"avoid"`
	Cases xyz.Case[Expression, []Expression]            `json:"cases"`
	Group xyz.Case[Expression, []Expression]            `json:"group"`
}]

var Expressions = xyz.AccessorFor(Expression.Values)

type Modification xyz.Tagged[any, struct {
	Set xyz.Case[Modification, xyz.Pair[Column, Value]] `json:"set"`
	Arr xyz.Case[Modification, []Modification]          `json:"arr"`
}]

var Modifications = xyz.AccessorFor(Modification.Values)

type Patch []Modification

type Stats []Calculation

type Calculation xyz.Tagged[any, struct {
	Add Calculation                   `json:"add"`
	Sum xyz.Case[Calculation, Column] `json:"sum"`
	Avg xyz.Case[Calculation, Column] `json:"avg"`
	Top xyz.Case[Calculation, Column] `json:"top"`
	Min xyz.Case[Calculation, Column] `json:"min"`
	Max xyz.Case[Calculation, Column] `json:"max"`
}]

var Calculations = xyz.AccessorFor(Calculation.Values)
