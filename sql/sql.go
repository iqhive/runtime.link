package sql

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"runtime.link/api/xray"
	"runtime.link/sql/std/sodium"
	"runtime.link/xyz"
)

const (
	ErrDuplicate        = errorString("record already exists")
	ErrTransactionUsage = errorString("empty transaction level")
)

type UnsupportedTypeError struct {
	Type xyz.TypeOf[sodium.Value]
}

func (e UnsupportedTypeError) Error() string {
	return fmt.Sprintf("unsupported type: %v", e.Type)
}

type IncompatibleTypeError struct {
	Column sodium.Column
	Type   xyz.TypeOf[sodium.Value]
}

func (e IncompatibleTypeError) Error() string {
	return fmt.Sprintf("incompatible type: %v for column: %v", e.Type, e.Column.Name)
}

type errorString string

func (e errorString) Error() string {
	return string(e)
}

type counts interface{ clause() }

type modify interface{ modify() }

// BatchFunc that performs a collection of meaningfully grouped
// operations on a [Map].
type BatchFunc func(context.Context) error

// QueryFunc that returns a [Query] for the given key and value.
type QueryFunc[K comparable, V any] func(*K, *V) Query

// StatsFunc that returns a [Stats] for the given key and value.
type StatsFunc[K comparable, V any] func(*K, *V) Stats

// PatchFunc that returns a [Patch] for the given value.
type PatchFunc[V any] func(*V) Patch

// CheckFunc used for atomic operations.
type CheckFunc[V any] func(*V) Check

// Chan streams results from a [Map.Search] operation.
type Chan[K comparable, V any] chan xyz.Trio[K, V, error]

type result[K comparable, V any] struct {
	key K
	val V
	err error
}

func (r result[K, V]) Get() (K, V, error) {
	return r.key, r.val, r.err
}

type whereable interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~int |
		~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint | ~uintptr |
		~float32 | ~float64
}

type orderable interface {
	whereable | ~string | time.Time
}

func columnOf(ptr any) []sodium.Column {
	smutex.RLock()
	defer smutex.RUnlock()
	return mirror[ptr]
}

// Flag that determines the behaviour of an [Insert].
type Flag bool

const (
	Create Flag = false // means the insert will fail if the value already exists.
	Upsert Flag = true  // means the insert will overwrite the existing value if it exists.
)

type Query []sodium.Expression
type Stats []Counter
type Patch []sodium.Modification
type Check []sodium.Expression

func normalise(rvalue reflect.Value) sodium.Value {
	var comparable sodium.Value
	switch rvalue.Kind() {
	case reflect.Int8:
		comparable = sodium.Values.Int8.As(int8(rvalue.Int()))
	case reflect.Int16:
		comparable = sodium.Values.Int16.As(int16(rvalue.Int()))
	case reflect.Int32:
		comparable = sodium.Values.Int32.As(int32(rvalue.Int()))
	case reflect.Int64, reflect.Int:
		comparable = sodium.Values.Int64.As(int64(rvalue.Int()))
	case reflect.Uint8:
		comparable = sodium.Values.Uint8.As(uint8(rvalue.Uint()))
	case reflect.Uint16:
		comparable = sodium.Values.Uint16.As(uint16(rvalue.Uint()))
	case reflect.Uint32:
		comparable = sodium.Values.Uint32.As(uint32(rvalue.Uint()))
	case reflect.Uint64, reflect.Uint, reflect.Uintptr:
		comparable = sodium.Values.Uint64.As(uint64(rvalue.Uint()))
	case reflect.Float32:
		comparable = sodium.Values.Float32.As(float32(rvalue.Float()))
	case reflect.Float64:
		comparable = sodium.Values.Float64.As(float64(rvalue.Float()))
	case reflect.String:
		comparable = sodium.Values.String.As(rvalue.String())
	case reflect.Struct:
		if rvalue.Type() == reflect.TypeOf(time.Time{}) {
			comparable = sodium.Values.Time.As(rvalue.Interface().(time.Time))
			break
		}
	}
	return comparable
}

// Index returns a new [sodium.Expression] that can be used inside a [QueryFunc]
// to refer to one of the columns in the table. The ptr must point inside the
// arguments passed to the [QueryFunc].
func Index[V comparable](ptr *V) struct {
	Equals func(V) sodium.Expression // matches values that are equal to the given value.
} {
	return struct {
		Equals func(V) sodium.Expression
	}{
		Equals: func(val V) sodium.Expression {
			columns := columnOf(ptr)
			if len(columns) > 0 {
				var values = ValuesOf(val)
				var equality []sodium.Expression
				for i, column := range columns {
					equality = append(equality, sodium.Expressions.Index.As(
						xyz.NewPair(column, values[i]),
					))
				}
				return sodium.Expressions.Group.As(equality)
			}
			return sodium.Expressions.Index.As(
				xyz.NewPair(columns[0], normalise(reflect.ValueOf(val))),
			)
		},
	}
}

type countable interface {
	atomic.Int32 | atomic.Int64 | atomic.Uint32 | atomic.Uint64
}

// Count returns a new counter for the given pointer, it can be used inside a
// [StatsFunc]. The ptr will be incremented by the given value.
func Count[V countable](ptr *V) Counter {
	return counter[V]{
		ptr:  ptr,
		calc: sodium.Calculations.Add,
	}
}

type counter[V countable] struct {
	ptr  *V
	calc sodium.Calculation
}

func (counter[T]) count() {}

// Counter is a type that can be used inside a [StatsFunc] to calculate a
// sum values.
type Counter interface {
	count()
}

// Set returns a new [sodium.Modification] that can be used inside a [PatchFunc]
// to refer to one of the columns in the table. The ptr must point inside the
// arguments passed to the [PatchFunc].
func Set[V comparable](ptr *V, val V) sodium.Modification {
	columns := columnOf(ptr)
	if len(columns) > 0 {
		var values = ValuesOf(val)
		var modifications []sodium.Modification
		for i, column := range columns {
			modifications = append(modifications, sodium.Modifications.Set.As(
				xyz.NewPair(column, values[i]),
			))
		}
		return sodium.Modifications.Arr.As(modifications)
	}
	return sodium.Modifications.Set.As(
		xyz.NewPair(columns[0], normalise(reflect.ValueOf(val))),
	)
}

// Where returns a new [WhereExpression] for the given pointer, it can be
// used inside a [QueryFunc] to refer to one of the columns in the table.
// The ptr must point inside the arguments passed to the [QueryFunc].
func Where[V whereable](ptr *V) struct {
	Min func(V) sodium.Expression // matches values greater than or equal to the given value.
	Max func(V) sodium.Expression // matches values less than or equal to the given value.

	MoreThan func(V) sodium.Expression // matches values greater than the given value.
	LessThan func(V) sodium.Expression // matches values less than the given value.
} {
	normalise := func(as func(xyz.Pair[sodium.Column, sodium.Value]) sodium.WhereExpression, rvalue reflect.Value) sodium.WhereExpression {
		var ordered sodium.Value
		switch rvalue.Kind() {
		case reflect.Int8:
			ordered = sodium.Values.Int8.As(int8(rvalue.Int()))
		case reflect.Int16:
			ordered = sodium.Values.Int16.As(int16(rvalue.Int()))
		case reflect.Int32:
			ordered = sodium.Values.Int32.As(int32(rvalue.Int()))
		case reflect.Int64, reflect.Int:
			ordered = sodium.Values.Int64.As(int64(rvalue.Int()))
		case reflect.Uint8:
			ordered = sodium.Values.Uint8.As(uint8(rvalue.Uint()))
		case reflect.Uint16:
			ordered = sodium.Values.Uint16.As(uint16(rvalue.Uint()))
		case reflect.Uint32:
			ordered = sodium.Values.Uint32.As(uint32(rvalue.Uint()))
		case reflect.Uint64, reflect.Uint, reflect.Uintptr:
			ordered = sodium.Values.Uint64.As(uint64(rvalue.Uint()))
		case reflect.Float32:
			ordered = sodium.Values.Float32.As(float32(rvalue.Float()))
		case reflect.Float64:
			ordered = sodium.Values.Float64.As(float64(rvalue.Float()))
		case reflect.String:
			ordered = sodium.Values.String.As(rvalue.String())
		case reflect.Struct:
			if rvalue.Type() == reflect.TypeOf(time.Time{}) {
				ordered = sodium.Values.Time.As(rvalue.Interface().(time.Time))
				break
			}
		}
		return as(xyz.NewPair(columnOf(ptr)[0], ordered))
	}
	return struct {
		Min func(V) sodium.Expression
		Max func(V) sodium.Expression

		MoreThan func(V) sodium.Expression
		LessThan func(V) sodium.Expression
	}{
		Min: func(val V) sodium.Expression {
			return sodium.Expressions.Where.As(normalise(sodium.WhereExpressions.Min.As, reflect.ValueOf(val)))
		},
		Max: func(val V) sodium.Expression {
			return sodium.Expressions.Where.As(normalise(sodium.WhereExpressions.Max.As, reflect.ValueOf(val)))
		},
		MoreThan: func(val V) sodium.Expression {
			return sodium.Expressions.Where.As(normalise(sodium.WhereExpressions.MoreThan.As, reflect.ValueOf(val)))
		},
		LessThan: func(val V) sodium.Expression {
			return sodium.Expressions.Where.As(normalise(sodium.WhereExpressions.LessThan.As, reflect.ValueOf(val)))
		},
	}
}

// Match returns a new [MatchExpression] for the given pointer, it can be
// used inside a [QueryFunc] to refer to one of the columns in the table.
// The ptr must point inside the arguments passed to the [QueryFunc].
func Match[V ~string](ptr *V) struct {
	Contains  func(string) sodium.Expression // matches values that contain the given string.
	HasPrefix func(string) sodium.Expression // matches values that start with the given string.
	HasSuffix func(string) sodium.Expression // matches values that end with the given string.
} {
	return struct {
		Contains  func(string) sodium.Expression
		HasPrefix func(string) sodium.Expression
		HasSuffix func(string) sodium.Expression
	}{
		Contains: func(val string) sodium.Expression {
			return sodium.Expressions.Match.As(
				sodium.MatchExpressions.Contains.As(xyz.NewPair(columnOf(ptr)[0], val)),
			)
		},
		HasPrefix: func(val string) sodium.Expression {
			return sodium.Expressions.Match.As(
				sodium.MatchExpressions.HasPrefix.As(xyz.NewPair(columnOf(ptr)[0], val)))
		},
		HasSuffix: func(val string) sodium.Expression {
			return sodium.Expressions.Match.As(
				sodium.MatchExpressions.HasSuffix.As(xyz.NewPair(columnOf(ptr)[0], val)),
			)
		},
	}
}

// Order returns a new [OrderExpression] for the given pointer, it can be
// used inside a [QueryFunc] to refer to one of the columns in the table.
// The ptr must point inside the arguments passed to the [QueryFunc].
func Order[V orderable](ptr *V) struct {
	Increasing func() sodium.Expression // orders values in increasing order.
	Decreasing func() sodium.Expression // orders values in decreasing order.
} {
	return struct {
		Increasing func() sodium.Expression
		Decreasing func() sodium.Expression
	}{
		Increasing: func() sodium.Expression {
			return sodium.Expressions.Order.As(sodium.OrderExpressions.Increasing.As(columnOf(ptr)[0]))
		},
		Decreasing: func() sodium.Expression {
			return sodium.Expressions.Order.As(sodium.OrderExpressions.Decreasing.As(columnOf(ptr)[0]))
		},
	}
}

// Slice returns a new [RangeExpression] that can be used inside a [QueryFunc] to
// limit the affect of the query to a specific range of values. The from and upto
// values are zero based, and the range is half open, meaning that the value at
// the from index is included, but the value at the upto index is not.
func Slice(from, upto int) sodium.Expression {
	return sodium.Expressions.Range.As(sodium.Range{From: from, Upto: upto})
}

// Empty returns a new [Expression] for the given ptr that can be used inside a
// [QueryFunc] to filter for results that have an empty value at the given column.
func Empty[V any](ptr *V) sodium.Expression {
	return sodium.Expressions.Empty.As(columnOf(ptr)[0])
}

// Avoid returns a new [Expression] that can be used inside a [QueryFunc] to
// filter for results that do not match the given expression.
func Avoid(expr sodium.Expression) sodium.Expression {
	return sodium.Expressions.Avoid.As(expr)
}

// Cases returns a new [Expression] that can be used inside a [QueryFunc] to
// filter for results that match any of the given expressions.
func Cases(exprs ...sodium.Expression) sodium.Expression {
	return sodium.Expressions.Cases.As(exprs)
}

// ValuesOf destructures a Go value into a set of [Value]s.
// Nested struct fields and array elements are flattened into
// a single sequential slice of values. Pointers, complex values
// maps, functions, and slices will raise a panic if they are
// encountered (except for []byte). Unexported fields are ignored.
func ValuesOf(val any) []sodium.Value {
	var values []sodium.Value
	rvalue := reflect.ValueOf(val)
	for rvalue.Kind() == reflect.Pointer {
		rvalue = rvalue.Elem()
	}
	switch kind := rvalue.Kind(); kind {
	case reflect.Bool:
		values = append(values, sodium.Values.Bool.As(rvalue.Bool()))
	case reflect.Int8:
		values = append(values, sodium.Values.Int8.As(int8(rvalue.Int())))
	case reflect.Int16:
		values = append(values, sodium.Values.Int16.As(int16(rvalue.Int())))
	case reflect.Int32:
		values = append(values, sodium.Values.Int32.As(int32(rvalue.Int())))
	case reflect.Int64:
		values = append(values, sodium.Values.Int64.As(int64(rvalue.Int())))
	case reflect.Int:
		values = append(values, sodium.Values.Int64.As(int64(rvalue.Int())))
	case reflect.Uint8:
		values = append(values, sodium.Values.Uint8.As(uint8(rvalue.Uint())))
	case reflect.Uint16:
		values = append(values, sodium.Values.Uint16.As(uint16(rvalue.Uint())))
	case reflect.Uint32:
		values = append(values, sodium.Values.Uint32.As(uint32(rvalue.Uint())))
	case reflect.Uint64, reflect.Uint, reflect.Uintptr:
		values = append(values, sodium.Values.Uint64.As(uint64(rvalue.Uint())))
	case reflect.Float32:
		values = append(values, sodium.Values.Float32.As(float32(rvalue.Float())))
	case reflect.Float64:
		values = append(values, sodium.Values.Float64.As(float64(rvalue.Float())))
	case reflect.String:
		values = append(values, sodium.Values.String.As(rvalue.String()))
	case reflect.Slice:
		if rvalue.Type().Elem().Kind() == reflect.Uint8 {
			values = append(values, sodium.Values.Bytes.As(rvalue.Bytes()))
			break
		}
		fallthrough
	case reflect.Struct:
		if rvalue.Type() == reflect.TypeOf(time.Time{}) {
			values = append(values, sodium.Values.Time.As(rvalue.Interface().(time.Time)))
			break
		}
		for i := 0; i < rvalue.NumField(); i++ {
			field := rvalue.Field(i)
			if !field.CanInterface() {
				continue
			}
			values = append(values, ValuesOf(field.Interface())...)
		}
	case reflect.Array:
		for i := 0; i < rvalue.Len(); i++ {
			values = append(values, ValuesOf(rvalue.Index(i).Interface())...)
		}
	default:
		panic("sql.ValuesOf: unsupported kind " + kind.String())
	}
	return values
}

// columnsOf behaviour is documented in [Open], candidate
// function to export (what function signature? generic?).
func columnsOf(field reflect.StructField, path ...string) []sodium.Column {
	var column sodium.Column
	column.Name = strings.ToLower(field.Name)
	if tag := field.Tag.Get("txt"); tag != "" {
		column.Name = tag
	}
	if tag := field.Tag.Get("sql"); tag != "" {
		column.Name = tag
	}
	if len(path) > 0 {
		column.Name = strings.Join(path, "_") + "_" + column.Name
	}
	column.Tags = field.Tag
	switch kind := field.Type.Kind(); kind {
	case reflect.Bool:
		column.Type = sodium.Values.Bool
	case reflect.Int8:
		column.Type = sodium.Values.Int8
	case reflect.Int16:
		column.Type = sodium.Values.Int16
	case reflect.Int32:
		column.Type = sodium.Values.Int32
	case reflect.Int64:
		column.Type = sodium.Values.Int64
	case reflect.Int:
		column.Type = sodium.Values.Int64
	case reflect.Uint8:
		column.Type = sodium.Values.Uint8
	case reflect.Uint16:
		column.Type = sodium.Values.Uint16
	case reflect.Uint32:
		column.Type = sodium.Values.Uint32
	case reflect.Uint64, reflect.Uint, reflect.Uintptr:
		column.Type = sodium.Values.Uint64
	case reflect.Float32:
		column.Type = sodium.Values.Float32
	case reflect.Float64:
		column.Type = sodium.Values.Float64
	case reflect.String:
		column.Type = sodium.Values.String
	case reflect.Struct:
		if field.Type == reflect.TypeOf(time.Time{}) {
			column.Type = sodium.Values.Time
			break
		}
		var columns []sodium.Column
		for i := 0; i < field.Type.NumField(); i++ {
			promote := append(path, column.Name)
			if field.Type.Field(i).Anonymous {
				promote = path
			}
			columns = append(columns, columnsOf(field.Type.Field(i), promote...)...)
		}
		return columns
	case reflect.Array:
		var columns []sodium.Column
		for i := 0; i < field.Type.Len(); i++ {
			columns = append(columns, columnsOf(field.Type.Field(i), append(path, strconv.Itoa(i+1))...)...)
		}
		return columns
	case reflect.Slice:
		if field.Type.Elem().Kind() == reflect.Uint8 {
			column.Type = sodium.Values.Bytes
			break
		}
		fallthrough
	default:
		panic("sql.columnsOf: unsupported kind " + kind.String())
	}
	return []sodium.Column{column}
}

func decode(ptr reflect.Value, values []sodium.Value) []sodium.Value {
	var value = ptr.Elem()
	if value.Type().Size() == 0 {
		return values
	}
	switch kind := value.Kind(); kind {
	case reflect.Bool:
		value.SetBool(sodium.Values.Bool.Get(values[0]))
	case reflect.Int8:
		value.SetInt(int64(sodium.Values.Int8.Get(values[0])))
	case reflect.Int16:
		value.SetInt(int64(sodium.Values.Int16.Get(values[0])))
	case reflect.Int32:
		value.SetInt(int64(sodium.Values.Int32.Get(values[0])))
	case reflect.Int64, reflect.Int:
		value.SetInt(int64(sodium.Values.Int64.Get(values[0])))
	case reflect.Uint8:
		value.SetUint(uint64(sodium.Values.Uint8.Get(values[0])))
	case reflect.Uint16:
		value.SetUint(uint64(sodium.Values.Uint16.Get(values[0])))
	case reflect.Uint32:
		value.SetUint(uint64(sodium.Values.Uint32.Get(values[0])))
	case reflect.Uint64, reflect.Uint, reflect.Uintptr:
		value.SetUint(uint64(sodium.Values.Uint64.Get(values[0])))
	case reflect.Float32:
		value.SetFloat(float64(sodium.Values.Float32.Get(values[0])))
	case reflect.Float64:
		value.SetFloat(float64(sodium.Values.Float64.Get(values[0])))
	case reflect.String:
		value.SetString(sodium.Values.String.Get(values[0]))
	case reflect.Slice:
		if value.Type().Elem().Kind() == reflect.Uint8 {
			value.SetBytes(sodium.Values.Bytes.Get(values[0]))
			break
		}
	case reflect.Struct:
		if value.Type() == reflect.TypeOf(time.Time{}) {
			value.Set(reflect.ValueOf(sodium.Values.Time.Get(values[0])))
			break
		}
		for i := 0; i < value.NumField(); i++ {
			values = decode(value.Field(i).Addr(), values)
		}
	case reflect.Array:
		for i := 0; i < value.Len(); i++ {
			values = decode(value.Index(i).Addr(), values)
		}
	}
	if len(values) > 0 {
		values = values[1:]
	}
	return values
}

// NewResult returns a slice of sodium values for the columns of the given table,
// a scanner function must be provided, that behaves like a [database/sql] scanner.
func NewResult(table sodium.Table, scanner func(...any) error) ([]sodium.Value, error) {
	var ptrs []any
	for _, column := range table.Index {
		ptrs = append(ptrs, newPointerFor(column))
	}
	for _, column := range table.Value {
		ptrs = append(ptrs, newPointerFor(column))
	}
	if err := scanner(ptrs...); err != nil {
		return nil, xray.Error(err)
	}
	var values []sodium.Value
	for _, ptr := range ptrs {
		values = append(values, ValuesOf(ptr)...)
	}
	return values, nil
}

// NewOutput returns a slice of sodium values for the given sodium calculations.
// A scanner function must be provided, that behaves like a [database/sql] scanner.
func NewOutput(calcs []sodium.Calculation, scanner func(...any) error) ([]sodium.Value, error) {
	var ptrs []any
	for _, calc := range calcs {
		switch calc {
		case sodium.Calculations.Add:
			ptrs = append(ptrs, new(uint64))
		default:
			switch xyz.ValueOf(calc) {
			case sodium.Calculations.Sum:
				column := sodium.Calculations.Sum.Get(calc)
				ptrs = append(ptrs, newPointerFor(column))
			case sodium.Calculations.Avg:
				column := sodium.Calculations.Avg.Get(calc)
				ptrs = append(ptrs, newPointerFor(column))
			case sodium.Calculations.Max:
				column := sodium.Calculations.Max.Get(calc)
				ptrs = append(ptrs, newPointerFor(column))
			case sodium.Calculations.Min:
				column := sodium.Calculations.Min.Get(calc)
				ptrs = append(ptrs, newPointerFor(column))
			case sodium.Calculations.Top:
				column := sodium.Calculations.Top.Get(calc)
				ptrs = append(ptrs, newPointerFor(column))
			default:
				panic("sql.NewOutput: unsupported calculation " + calc.String())
			}
		}
	}
	if err := scanner(ptrs...); err != nil {
		return nil, xray.Error(err)
	}
	var values []sodium.Value
	for _, ptr := range ptrs {
		values = append(values, ValuesOf(ptr)...)
	}
	return values, nil
}

// newPointerFor returns a new pointer for the given column, suitable for
// use in a 'Scan' function.
func newPointerFor(column sodium.Column) any {
	switch column.Type {
	case sodium.Values.Bool:
		return new(bool)
	case sodium.Values.Int8:
		return new(int8)
	case sodium.Values.Int16:
		return new(int16)
	case sodium.Values.Int32:
		return new(int32)
	case sodium.Values.Int64:
		return new(int64)
	case sodium.Values.Uint8:
		return new(uint8)
	case sodium.Values.Uint16:
		return new(uint16)
	case sodium.Values.Uint32:
		return new(uint32)
	case sodium.Values.Uint64:
		return new(uint64)
	case sodium.Values.Float32:
		return new(float32)
	case sodium.Values.Float64:
		return new(float64)
	case sodium.Values.String:
		return new(string)
	case sodium.Values.Bytes:
		return new([]byte)
	case sodium.Values.Time:
		return new(time.Time)
	default:
		panic("sql.NewPointerFor: unsupported type " + column.Type.String())
	}
}
