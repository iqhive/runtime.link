// Package sql defines a map type with support for structured queries.
package sql

import (
	"context"
	"reflect"
	"sync"
	"time"

	"runtime.link/xyz"
)

// Table represents a
type Table struct {
	_ struct{}

	Name string

	Index []Column
	Value []Column
}

type Column struct {
	_ struct{}

	Name string
	Type xyz.TypeOf[Value]
	Tags reflect.StructTag
}

type clause interface{ clause() }

type Query []Expression

// Range is an half open range (or slice) on the data
// to be operated on. Index starts at 0. Range{0,1}
// means the first element, Range{0,2} means the first
// two elements, etc.
type Range struct {
	From int
	Upto int
}

type counts interface{ clause() }

type Stats []counts

type Patch []modify

type modify interface{ modify() }

// Transaction flags.
type Transaction uint64

const (
	DirtyReads Transaction = 1 << iota // means the transaction can read uncommitted data.
	ReadWrites                         // means the transaction can read after it writes.
	Sequential                         // means that all operations will be executed in order.
	LockWrites                         // means that all writes will be locked until the transaction ends.
	GlobalLock                         // means that the database will be locked until the transaction ends.
	Repeatable                         // means that the transaction can be repeated until it succeeds.
)

// BatchFunc that performs a collection of meaningfully grouped
// operations on a [Map].
type BatchFunc func(context.Context) error

// Command is a database operation that can be executed asynchronously.
type Command interface {
	Wait() (int, error)
}

// Flag that determines the behaviour of an [Insert].
type Flag bool

const (
	Create Flag = false // means the insert will fail if the value already exists.
	Upsert              // means the insert will overwrite the existing value if it exists.
)

// Database that supports SODIUM operations, can be used to implement a [Map].
type Database[SQL Command] interface {
	// Search the [Table] for [Value]s that match the given [Query]. Whenever a
	// result is found, the corresponding [Pointer] argument is filled with the
	// result and the given callback is called. If the callback returns an error,
	// the search is aborted and the operation fails.
	Search(context.Context, Table, Query, []Pointer, func() error) Command
	// Output calculates the requested [Stats] for the given table and
	// writes them into the respective [Stats] values.
	Output(context.Context, Table, Stats) Command
	// Delete should remove any records that match the given query from
	// the table. A finite [Range] must be specified, if the [Range] is
	// empty, the operation will fail.
	Delete(context.Context, Table, Query, Range) Command
	// Insert a [Value] into the table. If the value already exists, the
	// flag determines whether the operation should fail (false) or overwrite
	// the existing value (true).
	Insert(context.Context, Table, []Value, Flag) Command
	// Update should apply the given patch to each [Value]s in
	// the table that matches the given [Query]. A finite [Range]
	// must be specified, if the [Range] is empty, the operation will fail.
	Update(context.Context, Table, Query, Patch, Range) Command
	// Manage the execution of the given operations within a transaction level
	// specified by the given [Transaction]. An empty transaction or an empty
	// list of operations will result in a usage error.
	Manage(context.Context, Transaction, ...Command) error
}

// QueryFunc that returns a [Query] for the given key and value.
type QueryFunc[K comparable, V any] func(*K, *V) Query

// StatsFunc that returns a [Stats] for the given key and value.
type StatsFunc[K comparable, V any] func(*K, *V) Stats

// PatchFunc that returns a [Patch] for the given value.
type PatchFunc[V any] func(*V) Patch

// Chan streams results from a [Map.Search] operation.
type Chan[K comparable, V any] chan result[K, V]

type result[K comparable, V any] struct {
	key K
	val V
	err error
}

// Map represents a distinct mapping of data stored in a [Database].
type Map[K comparable, V any] struct {
	_ struct{}

	Search func(context.Context, QueryFunc[K, V]) Chan[K, V]
	Output func(context.Context, StatsFunc[K, V]) error
	Delete func(context.Context, QueryFunc[K, V]) (int, error)
	Insert func(context.Context, K, Flag, V) error
	Update func(context.Context, QueryFunc[K, V], PatchFunc[V]) (int, error)
	Manage func(context.Context, Transaction, BatchFunc) error
}

// Comparable values.
type Comparable xyz.Switch[any, struct {
	Bool    xyz.Case[Comparable, bool]
	Int8    xyz.Case[Comparable, int8]
	Int16   xyz.Case[Comparable, int16]
	Int32   xyz.Case[Comparable, int32]
	Int64   xyz.Case[Comparable, int64]
	Int     xyz.Case[Comparable, int]
	Uint8   xyz.Case[Comparable, uint8]
	Uint16  xyz.Case[Comparable, uint16]
	Uint32  xyz.Case[Comparable, uint32]
	Uint64  xyz.Case[Comparable, uint64]
	Uint    xyz.Case[Comparable, uint]
	Uintptr xyz.Case[Comparable, uintptr]
	Float32 xyz.Case[Comparable, float32]
	Float64 xyz.Case[Comparable, float64]
	String  xyz.Case[Comparable, string]
}]

var NewComparable = xyz.AccessorFor(Comparable.Values)

// Ordered values.
type Ordered xyz.Switch[uint64, struct {
	Int8    xyz.Case[Ordered, int8]
	Int16   xyz.Case[Ordered, int16]
	Int32   xyz.Case[Ordered, int32]
	Int64   xyz.Case[Ordered, int64]
	Int     xyz.Case[Ordered, int]
	Uint8   xyz.Case[Ordered, uint8]
	Uint16  xyz.Case[Ordered, uint16]
	Uint32  xyz.Case[Ordered, uint32]
	Uint64  xyz.Case[Ordered, uint64]
	Uint    xyz.Case[Ordered, uint]
	Uintptr xyz.Case[Ordered, uintptr]
	Float32 xyz.Case[Ordered, float32]
	Float64 xyz.Case[Ordered, float64]
	Time    xyz.Case[Ordered, time.Time]
}]

var NewOrdered = xyz.AccessorFor(Ordered.Values)

// WhereExpression within a [Query].
type WhereExpression xyz.Switch[any, struct {
	Min xyz.Case[WhereExpression, xyz.Pair[Column, Ordered]]
	Max xyz.Case[WhereExpression, xyz.Pair[Column, Ordered]]

	MoreThan xyz.Case[WhereExpression, xyz.Pair[Column, Ordered]]
	LessThan xyz.Case[WhereExpression, xyz.Pair[Column, Ordered]]
}]

var WhereExpressions = xyz.AccessorFor(WhereExpression.Values)

// MatchExpression within a [Query].
type MatchExpression xyz.Switch[any, struct {
	Contains  xyz.Case[MatchExpression, xyz.Pair[Column, string]]
	HasPrefix xyz.Case[MatchExpression, xyz.Pair[Column, string]]
	HasSuffix xyz.Case[MatchExpression, xyz.Pair[Column, string]]
}]

var MatchExpressions = xyz.AccessorFor(MatchExpression.Values)

// OrderExpression within a [Query].
type OrderExpression xyz.Switch[any, struct {
	Increasing xyz.Case[OrderExpression, Column]
	Decreasing xyz.Case[OrderExpression, Column]
}]

var OrderExpressions = xyz.AccessorFor(OrderExpression.Values)

// Expression within a [Query].
type Expression xyz.Switch[any, struct {
	Index xyz.Case[Expression, xyz.Pair[Column, Comparable]]
	Where xyz.Case[Expression, WhereExpression]
	Match xyz.Case[Expression, MatchExpression]
	Order xyz.Case[Expression, OrderExpression]
	Range xyz.Case[Expression, Range]
	Empty xyz.Case[Expression, Column]
	Avoid xyz.Case[Expression, Expression]
	Cases xyz.Case[Expression, []Expression]
}]

var Expressions = xyz.AccessorFor(Expression.Values)

type Modification xyz.Switch[any, struct {
	Set xyz.Case[Modification, xyz.Pair[Column, Value]]
}]

type Value xyz.Switch[any, struct {
	Bool    xyz.Case[Value, bool]
	Int8    xyz.Case[Value, int8]
	Int16   xyz.Case[Value, int16]
	Int32   xyz.Case[Value, int32]
	Int64   xyz.Case[Value, int64]
	Int     xyz.Case[Value, int]
	Uint8   xyz.Case[Value, uint8]
	Uint16  xyz.Case[Value, uint16]
	Uint32  xyz.Case[Value, uint32]
	Uint64  xyz.Case[Value, uint64]
	Uint    xyz.Case[Value, uint]
	Uintptr xyz.Case[Value, uintptr]
	Float32 xyz.Case[Value, float32]
	Float64 xyz.Case[Value, float64]
	String  xyz.Case[Value, string]
	Time    xyz.Case[Value, time.Time]
}]

var Values = xyz.AccessorFor(Value.Values)

type Pointer xyz.Switch[any, struct {
	Bool    xyz.Case[Pointer, *bool]
	Int8    xyz.Case[Pointer, *int8]
	Int16   xyz.Case[Pointer, *int16]
	Int32   xyz.Case[Pointer, *int32]
	Int64   xyz.Case[Pointer, *int64]
	Int     xyz.Case[Pointer, *int]
	Uint8   xyz.Case[Pointer, *uint8]
	Uint16  xyz.Case[Pointer, *uint16]
	Uint32  xyz.Case[Pointer, *uint32]
	Uint64  xyz.Case[Pointer, *uint64]
	Uint    xyz.Case[Pointer, *uint]
	Uintptr xyz.Case[Pointer, *uintptr]
	Float32 xyz.Case[Pointer, *float32]
	Float64 xyz.Case[Pointer, *float64]
	String  xyz.Case[Pointer, *string]
	Time    xyz.Case[Pointer, *time.Time]
}]

var Pointers = xyz.AccessorFor(Pointer.Values)

type whereable interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~int |
		~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint | ~uintptr |
		~float32 | ~float64
}

type orderable interface {
	whereable | ~string | time.Time
}

var rmutex sync.RWMutex
var mirror = make(map[any]Column)

func columnOf(ptr any) Column {
	rmutex.RLock()
	defer rmutex.RUnlock()
	return mirror[ptr]
}

// Where returns a new [WhereExpression] for the given pointer, it can be
// used inside a [QueryFunc] to refer to one of the columns in the table.
// The ptr must point inside the arguments passed to the [QueryFunc].
func Where[V whereable](ptr *V) struct {
	Min func(V) Expression // matches values greater than or equal to the given value.
	Max func(V) Expression // matches values less than or equal to the given value.

	MoreThan func(V) Expression // matches values greater than the given value.
	LessThan func(V) Expression // matches values less than the given value.
} {
	normalise := func(as func(xyz.Pair[Column, Ordered]) WhereExpression, rvalue reflect.Value) WhereExpression {
		var ordered Ordered
		switch rvalue.Kind() {
		case reflect.Int8:
			ordered = NewOrdered.Int8.As(int8(rvalue.Int()))
		case reflect.Int16:
			ordered = NewOrdered.Int16.As(int16(rvalue.Int()))
		case reflect.Int32:
			ordered = NewOrdered.Int32.As(int32(rvalue.Int()))
		case reflect.Int64:
			ordered = NewOrdered.Int64.As(int64(rvalue.Int()))
		case reflect.Int:
			ordered = NewOrdered.Int.As(int(rvalue.Int()))
		case reflect.Uint8:
			ordered = NewOrdered.Uint8.As(uint8(rvalue.Uint()))
		case reflect.Uint16:
			ordered = NewOrdered.Uint16.As(uint16(rvalue.Uint()))
		case reflect.Uint32:
			ordered = NewOrdered.Uint32.As(uint32(rvalue.Uint()))
		case reflect.Uint64:
			ordered = NewOrdered.Uint64.As(uint64(rvalue.Uint()))
		case reflect.Uint:
			ordered = NewOrdered.Uint.As(uint(rvalue.Uint()))
		case reflect.Uintptr:
			ordered = NewOrdered.Uintptr.As(uintptr(rvalue.Uint()))
		case reflect.Float32:
			ordered = NewOrdered.Float32.As(float32(rvalue.Float()))
		case reflect.Float64:
			ordered = NewOrdered.Float64.As(float64(rvalue.Float()))
		}
		return as(xyz.NewPair(columnOf(ptr), ordered))
	}
	return struct {
		Min func(V) Expression
		Max func(V) Expression

		MoreThan func(V) Expression
		LessThan func(V) Expression
	}{
		Min: func(val V) Expression {
			return Expressions.Where.As(normalise(WhereExpressions.Min.As, reflect.ValueOf(val)))
		},
		Max: func(val V) Expression {
			return Expressions.Where.As(normalise(WhereExpressions.Max.As, reflect.ValueOf(val)))
		},
		MoreThan: func(val V) Expression {
			return Expressions.Where.As(normalise(WhereExpressions.MoreThan.As, reflect.ValueOf(val)))
		},
		LessThan: func(val V) Expression {
			return Expressions.Where.As(normalise(WhereExpressions.LessThan.As, reflect.ValueOf(val)))
		},
	}
}

// Match returns a new [MatchExpression] for the given pointer, it can be
// used inside a [QueryFunc] to refer to one of the columns in the table.
// The ptr must point inside the arguments passed to the [QueryFunc].
func Match[V string](ptr *V) struct {
	Contains  func(string) Expression // matches values that contain the given string.
	HasPrefix func(string) Expression // matches values that start with the given string.
	HasSuffix func(string) Expression // matches values that end with the given string.
} {
	return struct {
		Contains  func(string) Expression
		HasPrefix func(string) Expression
		HasSuffix func(string) Expression
	}{
		Contains: func(val string) Expression {
			return Expressions.Match.As(
				MatchExpressions.Contains.As(xyz.NewPair(columnOf(ptr), val)),
			)
		},
		HasPrefix: func(val string) Expression {
			return Expressions.Match.As(
				MatchExpressions.HasPrefix.As(xyz.NewPair(columnOf(ptr), val)))
		},
		HasSuffix: func(val string) Expression {
			return Expressions.Match.As(
				MatchExpressions.HasSuffix.As(xyz.NewPair(columnOf(ptr), val)),
			)
		},
	}
}

// Order returns a new [OrderExpression] for the given pointer, it can be
// used inside a [QueryFunc] to refer to one of the columns in the table.
// The ptr must point inside the arguments passed to the [QueryFunc].
func Order[V orderable](ptr *V) struct {
	Increasing func() Expression // orders values in increasing order.
	Decreasing func() Expression // orders values in decreasing order.
} {
	return struct {
		Increasing func() Expression
		Decreasing func() Expression
	}{
		Increasing: func() Expression {
			return Expressions.Order.As(OrderExpressions.Increasing.As(columnOf(ptr)))
		},
		Decreasing: func() Expression {
			return Expressions.Order.As(OrderExpressions.Decreasing.As(columnOf(ptr)))
		},
	}
}

// Slice returns a new [RangeExpression] that can be used inside a [QueryFunc] to
// limit the affect of the query to a specific range of values. The from and upto
// values are zero based, and the range is half open, meaning that the value at
// the from index is included, but the value at the upto index is not.
func Slice(from, upto int) Expression {
	return Expressions.Range.As(Range{From: from, Upto: upto})
}

// Empty returns a new [Expression] for the given ptr that can be used inside a
// [QueryFunc] to filter for results that have an empty value at the given column.
func Empty[V any](ptr *V) Expression {
	return Expressions.Empty.As(columnOf(ptr))
}

// Avoid returns a new [Expression] that can be used inside a [QueryFunc] to
// filter for results that do not match the given expression.
func Avoid(expr Expression) Expression {
	return Expressions.Avoid.As(expr)
}

// Cases returns a new [Expression] that can be used inside a [QueryFunc] to
// filter for results that match any of the given expressions.
func Cases(exprs ...Expression) Expression {
	return Expressions.Cases.As(exprs)
}
