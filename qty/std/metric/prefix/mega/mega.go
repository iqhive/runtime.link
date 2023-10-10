// Package mega provides standard types to represent millions for a SI unit.
package mega

import (
	"fmt"
	"math/big"

	"runtime.link/qty"
	"runtime.link/qty/std/physical"
)

// Bytes counts a digital storage in megabytes.
type Bytes uint64

// BytesFrom converts a quantity of digital storage to megabytes.
func BytesFrom(information qty.Measures[physical.Information]) Bytes {
	unit, factor, _ := information.Quantity()
	bits := unit.Bits.Mul(unit.Bits, factor)
	bytes := bits.Quo(bits, big.NewFloat(8e6))
	u64, _ := bytes.Uint64()
	return Bytes(u64)
}

// String implements fmt.Stringer.
func (MB Bytes) String() string { return fmt.Sprintf("%dMB", MB) }

// Quantity implements qty.Measures[physical.Information].
func (MB Bytes) Quantity() (physical.Information, *big.Float, string) {
	var f big.Float
	f.SetUint64(uint64(MB))
	return physical.Information{Bits: big.NewFloat(8e6)}, &f, "MB"
}

// Bits counts a digital storage in megabits.
type Bits float64

// BitsFrom converts a quantity of digital storage to megabits.
func BitsFrom(information qty.Measures[physical.Information]) Bits {
	unit, factor, _ := information.Quantity()
	bits := unit.Bits.Mul(unit.Bits, factor)
	bits = bits.Quo(bits, big.NewFloat(1e6))
	u64, _ := bits.Uint64()
	return Bits(u64)
}

// String implements fmt.Stringer.
func (Mb Bits) String() string { return fmt.Sprintf("%gMb", Mb) }

// Quantity implements qty.Measures[physical.Information].
func (Mb Bits) Quantity() (physical.Information, *big.Float, string) {
	var f big.Float
	f.SetUint64(uint64(Mb))
	return physical.Information{Bits: big.NewFloat(1e6)}, &f, "Mb"
}
