// Package qty provides a standard way to represent quantities with different underlying units.
package qty

import (
	"fmt"
	"math/big"

	"runtime.link/qty/std/measures"
)

// That Measures X is an interface that represents a quantity with an underlying standard unit of type X
type That[Measures Type[Measures]] interface {
	fmt.Stringer
	Quantity() (Measures, *big.Float, string)
}

// Type of quantity, defined
type Type[T any] interface {
	// perhaps relax this in future to enable users to define their own measure?
	measures.Brightness | measures.Distance | measures.Duration | measures.Current | measures.Information |
		measures.Mass | measures.Temperature | measures.Substance | measures.Resolution | measures.Area |
		measures.Volume | measures.Velocity | measures.Acceleration | measures.SurfaceDensity |
		measures.SpecificVolume | measures.CurrentDensity | measures.Force | measures.Pressure | measures.Energy |
		measures.Power | measures.Charge | measures.VolumeDensity | measures.MagneticFieldStrength |
		measures.ChemicalConcentration | measures.MassConcentration | measures.Luminance | measures.Frequency |
		measures.Voltage | measures.Capacitance | measures.Resistance | measures.Conductance |
		measures.Inductance | measures.MagneticFlux | measures.MagneticFluxDensity |
		measures.LuminousFlux | measures.Illuminance | measures.Radioactivity |
		measures.Catalysis | measures.RadiationAbsorbedDose | measures.RadiationEquivalentDose

	Float() *big.Float
	As(*big.Float) T
}

// Int quantity.
type Int[T Type[T], U That[T]] int

// Set the value of the quantity.
func (i *Int[T, U]) Set(val That[T]) {
	var measure U
	each, _, _ := measure.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Int64()
	*i = Int[T, U](i64)
}

// Quantity implements the [That[T]] interface.
func (i *Int[T, U]) Quantity() (T, *big.Float, string) {
	var kind T
	var measure U
	_, factor, symbol := measure.Quantity()
	var val big.Float
	val.SetInt64(int64(*i))
	return kind.As(&val), factor, symbol
}

// String implements the [fmt.Stringer] interface.
func (i Int[T, U]) String() string {
	var measure U
	_, _, symbol := measure.Quantity()
	return fmt.Sprintf("%d%s", i, symbol)
}

// Int8 quantity.
type Int8[T Type[T], U That[T]] int8

// Set the value of the quantity.
func (i *Int8[T, U]) Set(val That[T]) {
	var measure U
	each, _, _ := measure.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Int64()
	*i = Int8[T, U](i64)
}

// Quantity implements the [That[T]] interface.
func (i *Int8[T, U]) Quantity() (T, *big.Float, string) {
	var kind T
	var measure U
	_, factor, symbol := measure.Quantity()
	var val big.Float
	val.SetInt64(int64(*i))
	return kind.As(&val), factor, symbol
}

// String implements the [fmt.Stringer] interface.
func (i Int8[T, U]) String() string {
	var measure U
	_, _, symbol := measure.Quantity()
	return fmt.Sprintf("%d%s", i, symbol)
}

// Int16 quantity.
type Int16[T Type[T], U That[T]] int16

// Set the value of the quantity.
func (i *Int16[T, U]) Set(val That[T]) {
	var measure U
	each, _, _ := measure.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Int64()
	*i = Int16[T, U](i64)
}

// Quantity implements the [That[T]] interface.
func (i *Int16[T, U]) Quantity() (T, *big.Float, string) {
	var kind T
	var measure U
	_, factor, symbol := measure.Quantity()
	var val big.Float
	val.SetInt64(int64(*i))
	return kind.As(&val), factor, symbol
}

// String implements the [fmt.Stringer] interface.
func (i Int16[T, U]) String() string {
	var measure U
	_, _, symbol := measure.Quantity()
	return fmt.Sprintf("%d%s", i, symbol)
}

// Int32 quantity.
type Int32[T Type[T], U That[T]] int32

// Set the value of the quantity.
func (i *Int32[T, U]) Set(val That[T]) {
	var measure U
	each, _, _ := measure.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Int64()
	*i = Int32[T, U](i64)
}

// Quantity implements the [That[T]] interface.
func (i *Int32[T, U]) Quantity() (T, *big.Float, string) {
	var kind T
	var measure U
	_, factor, symbol := measure.Quantity()
	var val big.Float
	val.SetInt64(int64(*i))
	return kind.As(&val), factor, symbol
}

// String implements the [fmt.Stringer] interface.
func (i Int32[T, U]) String() string {
	var measure U
	_, _, symbol := measure.Quantity()
	return fmt.Sprintf("%d%s", i, symbol)
}

// Int64 quantity.
type Int64[T Type[T], U That[T]] int64

// Set the value of the quantity.
func (i *Int64[T, U]) Set(val That[T]) {
	var measure U
	each, _, _ := measure.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Int64()
	*i = Int64[T, U](i64)
}

// Quantity implements the [That[T]] interface.
func (i *Int64[T, U]) Quantity() (T, *big.Float, string) {
	var kind T
	var measure U
	_, factor, symbol := measure.Quantity()
	var val big.Float
	val.SetInt64(int64(*i))
	return kind.As(&val), factor, symbol
}

// String implements the [fmt.Stringer] interface.
func (i Int64[T, U]) String() string {
	var measure U
	_, _, symbol := measure.Quantity()
	return fmt.Sprintf("%d%s", i, symbol)
}

// Uint quantity.
type Uint[T Type[T], U That[T]] uint

// Set the value of the quantity.
func (i *Uint[T, U]) Set(val That[T]) {
	var measure U
	each, _, _ := measure.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Uint64()
	*i = Uint[T, U](i64)
}

// Quantity implements the [That[T]] interface.
func (i *Uint[T, U]) Quantity() (T, *big.Float, string) {
	var kind T
	var measure U
	_, factor, symbol := measure.Quantity()
	var val big.Float
	val.SetUint64(uint64(*i))
	return kind.As(&val), factor, symbol
}

// String implements the [fmt.Stringer] interface.
func (i Uint[T, U]) String() string {
	var measure U
	_, _, symbol := measure.Quantity()
	return fmt.Sprintf("%d%s", i, symbol)
}

// Uint8 quantity.
type Uint8[T Type[T], U That[T]] uint8

// Set the value of the quantity.
func (i *Uint8[T, U]) Set(val That[T]) {
	var measure U
	each, _, _ := measure.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Uint64()
	*i = Uint8[T, U](i64)
}

// Quantity implements the [That[T]] interface.
func (i *Uint8[T, U]) Quantity() (T, *big.Float, string) {
	var kind T
	var measure U
	_, factor, symbol := measure.Quantity()
	var val big.Float
	val.SetUint64(uint64(*i))
	return kind.As(&val), factor, symbol
}

// String implements the [fmt.Stringer] interface.
func (i Uint8[T, U]) String() string {
	var measure U
	_, _, symbol := measure.Quantity()
	return fmt.Sprintf("%d%s", i, symbol)
}

// Uint16 quantity.
type Uint16[T Type[T], U That[T]] uint16

// Set the value of the quantity.
func (i *Uint16[T, U]) Set(val That[T]) {
	var measure U
	each, _, _ := measure.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Uint64()
	*i = Uint16[T, U](i64)
}

// Quantity implements the [That[T]] interface.
func (i *Uint16[T, U]) Quantity() (T, *big.Float, string) {
	var kind T
	var measure U
	_, factor, symbol := measure.Quantity()
	var val big.Float
	val.SetUint64(uint64(*i))
	return kind.As(&val), factor, symbol
}

// String implements the [fmt.Stringer] interface.
func (i Uint16[T, U]) String() string {
	var measure U
	_, _, symbol := measure.Quantity()
	return fmt.Sprintf("%d%s", i, symbol)
}

// Uint32 quantity.
type Uint32[T Type[T], U That[T]] uint32

// Set the value of the quantity.
func (i *Uint32[T, U]) Set(val That[T]) {
	var measure U
	each, _, _ := measure.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Uint64()
	*i = Uint32[T, U](i64)
}

// Quantity implements the [That[T]] interface.
func (i *Uint32[T, U]) Quantity() (T, *big.Float, string) {
	var kind T
	var measure U
	_, factor, symbol := measure.Quantity()
	var val big.Float
	val.SetUint64(uint64(*i))
	return kind.As(&val), factor, symbol
}

// String implements the [fmt.Stringer] interface.
func (i Uint32[T, U]) String() string {
	var measure U
	_, _, symbol := measure.Quantity()
	return fmt.Sprintf("%d%s", i, symbol)
}

// Uint64 quantity.
type Uint64[T Type[T], U That[T]] uint64

// Set the value of the quantity.
func (i *Uint64[T, U]) Set(val That[T]) {
	var measure U
	each, _, _ := measure.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Uint64()
	*i = Uint64[T, U](i64)
}

// Quantity implements the [That[T]] interface.
func (i *Uint64[T, U]) Quantity() (T, *big.Float, string) {
	var kind T
	var measure U
	_, factor, symbol := measure.Quantity()
	var val big.Float
	val.SetUint64(uint64(*i))
	return kind.As(&val), factor, symbol
}

// String implements the [fmt.Stringer] interface.
func (i Uint64[T, U]) String() string {
	var measure U
	_, _, symbol := measure.Quantity()
	return fmt.Sprintf("%d%s", i, symbol)
}

// Uintptr quantity.
type Uintptr[T Type[T], U That[T]] uintptr

// Set the value of the quantity.
func (i *Uintptr[T, U]) Set(val That[T]) {
	var measure U
	each, _, _ := measure.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Uint64()
	*i = Uintptr[T, U](i64)
}

// Quantity implements the [That[T]] interface.
func (i *Uintptr[T, U]) Quantity() (T, *big.Float, string) {
	var kind T
	var measure U
	_, factor, symbol := measure.Quantity()
	var val big.Float
	val.SetUint64(uint64(*i))
	return kind.As(&val), factor, symbol
}

// String implements the [fmt.Stringer] interface.
func (i Uintptr[T, U]) String() string {
	var measure U
	_, _, symbol := measure.Quantity()
	return fmt.Sprintf("%d%s", i, symbol)
}

// Float32 quantity.
type Float32[T Type[T], U That[T]] float32

// Set the value of the quantity.
func (i *Float32[T, U]) Set(val That[T]) {
	var measure U
	each, _, _ := measure.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Float64()
	*i = Float32[T, U](i64)
}

// Quantity implements the [That[T]] interface.
func (i *Float32[T, U]) Quantity() (T, *big.Float, string) {
	var kind T
	var measure U
	_, factor, symbol := measure.Quantity()
	var val big.Float
	val.SetFloat64(float64(*i))
	return kind.As(&val), factor, symbol
}

// String implements the [fmt.Stringer] interface.
func (i Float32[T, U]) String() string {
	var measure U
	_, _, symbol := measure.Quantity()
	return fmt.Sprintf("%f%s", i, symbol)
}

// Float64 quantity.
type Float64[T Type[T], U That[T]] float64

// Set the value of the quantity.
func (i *Float64[T, U]) Set(val That[T]) {
	var measure U
	each, _, _ := measure.Quantity()

	unit, factor, _ := val.Quantity()
	base := unit.Float().Mul(unit.Float(), factor)
	base = base.Quo(base, each.Float())

	i64, _ := base.Float64()
	*i = Float64[T, U](i64)
}

// Quantity implements the [That[T]] interface.
func (i *Float64[T, U]) Quantity() (T, *big.Float, string) {
	var kind T
	var measure U
	_, factor, symbol := measure.Quantity()
	var val big.Float
	val.SetFloat64(float64(*i))
	return kind.As(&val), factor, symbol
}

// String implements the [fmt.Stringer] interface.
func (i Float64[T, U]) String() string {
	var measure U
	_, _, symbol := measure.Quantity()
	return fmt.Sprintf("%f%s", i, symbol)
}
