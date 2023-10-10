// Package qty provides a standard way to represent quantities with different underlying units.
package qty

import (
	"fmt"
	"math/big"

	"runtime.link/qty/std/measures"
)

type That[Measures Type[Measures]] interface {
	fmt.Stringer
	Quantity() (Measures, *big.Float, string)
}

type Type[T any] interface {
	// perhaps relax this in future to enable users to define their own measure?
	measures.Brightness | measures.Distance | measures.Duration | measures.Current | measures.Information | measures.Mass | measures.Temperature | measures.Substance

	Float() *big.Float
	As(*big.Float) T
}

type Int[T Type[T], U That[T]] int

func (i *Int[T, U]) Set(val That[T]) {
	var measure U
	each, _, _ := measure.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Int64()
	*i = Int[T, U](i64)
}

func (i *Int[Type, Measure]) Quantity() (Type, *big.Float, string) {
	var kind Type
	var measure Measure
	_, factor, symbol := measure.Quantity()
	var val big.Float
	val.SetInt64(int64(*i))
	return kind.As(&val), factor, symbol
}

func (i Int[Type, Measure]) String() string {
	var measure Measure
	_, _, symbol := measure.Quantity()
	return fmt.Sprintf("%d%s", i, symbol)
}
