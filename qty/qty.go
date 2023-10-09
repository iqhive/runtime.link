// Package qty provides a standard way to represent quantities with different underlying units.
package qty

import (
	"fmt"
	"math/big"
)

type Measures[T Of[T]] interface {
	fmt.Stringer
	Quantity() (T, *big.Float, string)
}

type Of[T any] interface {
	Float() *big.Float
	As(*big.Float) T
}

type Int[Type Of[Type], Unit Measures[Type]] int

func (i *Int[Type, Units]) Set(val Measures[Type]) {
	var generic Units
	each, _, _ := generic.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Int64()
	*i = Int[Type, Units](i64)
}

func (i *Int[Type, Units]) Quantity() (Type, *big.Float, string) {
	var kind Type
	var unit Units
	_, factor, symbol := unit.Quantity()
	var val big.Float
	val.SetInt64(int64(*i))
	return kind.As(&val), factor, symbol
}

func (i Int[Type, Units]) String() string {
	var unit Units
	_, _, s := unit.Quantity()
	return fmt.Sprintf("%d%s", i, s)
}
