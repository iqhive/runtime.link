// Package sodium provides a specification for the SODIUM standard database interface.
package sodium

import (
	"context"
	"reflect"
	"time"

	"runtime.link/xyz"
)

// Database that supports the SODIUM interface.
type Database interface {
	// Search the specified [Table] for [Value]s that match the given [Query].
	// Whenever a result is found, it is sent to the specified channel, which
	// is closed when the search is complete. The search will be cancelled if
	// the managed context is cancelled. The sends are non-blocking, so ensure
	// that the channel is buffered appropriately to avoid errors.
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
	// the existing value (true). Cannot be cancelled. The resulting keys and
	// values will be written into the provided slices.
	Insert(Table, []Value, bool, []Value) Job
	// Update should apply the given patch to each [Value]s in
	// the table that matches the given [Query]. A finite [Range]
	// must be specified, if the [Range] is empty, the operation will fail.
	//  Cannot be cancelled.
	Update(Table, Query, Patch) Job
	// Manage returns a channel that will manage the execution of the given jobs
	// within the transaction level specified by the given [Transaction]. Close
	// the channel to commit the transaction, or send a nil [Job] (or cancel the
	// context) to rollback the transaction.
	Manage(context.Context, Transaction) (chan<- Job, error)
}

type Value xyz.Tagged[any, struct {
	Bool    xyz.Case[Value, bool]
	Int8    xyz.Case[Value, int8]
	Int16   xyz.Case[Value, int16]
	Int32   xyz.Case[Value, int32]
	Int64   xyz.Case[Value, int64]
	Uint8   xyz.Case[Value, uint8]
	Uint16  xyz.Case[Value, uint16]
	Uint32  xyz.Case[Value, uint32]
	Uint64  xyz.Case[Value, uint64]
	Float32 xyz.Case[Value, float32]
	Float64 xyz.Case[Value, float64]
	String  xyz.Case[Value, string]
	Bytes   xyz.Case[Value, []byte]
	Time    xyz.Case[Value, time.Time]
}]

var Values = xyz.AccessorFor(Value.Values)

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

// Job of SQL job.
type Job interface {
	Wait(context.Context) (int, error)
}

// WhereExpression within a [Query].
type WhereExpression xyz.Tagged[any, struct {
	Min xyz.Case[WhereExpression, xyz.Pair[Column, Value]]
	Max xyz.Case[WhereExpression, xyz.Pair[Column, Value]]

	MoreThan xyz.Case[WhereExpression, xyz.Pair[Column, Value]]
	LessThan xyz.Case[WhereExpression, xyz.Pair[Column, Value]]
}]

var WhereExpressions = xyz.AccessorFor(WhereExpression.Values)

// MatchExpression within a [Query].
type MatchExpression xyz.Tagged[any, struct {
	Contains  xyz.Case[MatchExpression, xyz.Pair[Column, string]]
	HasPrefix xyz.Case[MatchExpression, xyz.Pair[Column, string]]
	HasSuffix xyz.Case[MatchExpression, xyz.Pair[Column, string]]
}]

var MatchExpressions = xyz.AccessorFor(MatchExpression.Values)

// OrderExpression within a [Query].
type OrderExpression xyz.Tagged[any, struct {
	Increasing xyz.Case[OrderExpression, Column]
	Decreasing xyz.Case[OrderExpression, Column]
}]

var OrderExpressions = xyz.AccessorFor(OrderExpression.Values)

// Expression within a [Query].
type Expression xyz.Tagged[any, struct {
	Value xyz.Case[Expression, bool]
	Index xyz.Case[Expression, xyz.Pair[Column, Value]]
	Where xyz.Case[Expression, WhereExpression]
	Match xyz.Case[Expression, MatchExpression]
	Order xyz.Case[Expression, OrderExpression]
	Range xyz.Case[Expression, Range]
	Empty xyz.Case[Expression, Column]
	Avoid xyz.Case[Expression, Expression]
	Cases xyz.Case[Expression, []Expression]
	Group xyz.Case[Expression, []Expression]
}]

var Expressions = xyz.AccessorFor(Expression.Values)

type Modification xyz.Tagged[any, struct {
	Set xyz.Case[Modification, xyz.Pair[Column, Value]]
	Arr xyz.Case[Modification, []Modification]
}]

var Modifications = xyz.AccessorFor(Modification.Values)

type Patch []Modification

type Stats []Calculation

type Calculation xyz.Tagged[any, struct {
	Add Calculation
	Sum xyz.Case[Calculation, Column]
	Avg xyz.Case[Calculation, Column]
	Top xyz.Case[Calculation, Column]
	Min xyz.Case[Calculation, Column]
	Max xyz.Case[Calculation, Column]
}]

var Calculations = xyz.AccessorFor(Calculation.Values)
