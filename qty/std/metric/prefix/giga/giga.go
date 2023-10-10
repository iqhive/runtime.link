// Package giga provides a standard way to represent billions for a SI unit.
package giga

import (
	"fmt"
	"math/big"

	"runtime.link/qty"
	"runtime.link/qty/std/measures"
)

// Bytes counts a digital storage in gigabytes.
type Bytes uint64

// BytesFrom converts a quantity of digital storage to gigabytes.
func BytesFrom(information qty.That[measures.Information]) Bytes {
	unit, factor, _ := information.Quantity()
	bits := unit.Bits.Mul(unit.Bits, factor)
	bytes := bits.Quo(bits, big.NewFloat(8e9))
	u64, _ := bytes.Uint64()
	return Bytes(u64)
}

// String implements fmt.Stringer
func (GB Bytes) String() string { return fmt.Sprintf("%dGB", GB) }

// Quantity implements qty.That[measures.Information].
func (GB Bytes) Quantity() (measures.Information, *big.Float, string) {
	var f big.Float
	f.SetUint64(uint64(GB))
	return measures.Information{Bits: big.NewFloat(8e9)}, &f, "GB"
}

// Bits counts a digital storage in gigabits.
type Bits uint64

// BitsFrom converts a quantity of digital storage to gigabits.
func BitsFrom(information qty.That[measures.Information]) Bits {
	unit, factor, _ := information.Quantity()
	bits := unit.Bits.Mul(unit.Bits, factor)
	bits = bits.Quo(bits, big.NewFloat(1e9))
	u64, _ := bits.Uint64()
	return Bits(u64)
}

// String implements fmt.Stringer.
func (Gb Bits) String() string { return fmt.Sprintf("%dGb", Gb) }

// Quantity implements qty.That[measures.Information].
func (Gb Bits) Quantity() (measures.Information, *big.Float, string) {
	var f big.Float
	f.SetUint64(uint64(Gb))
	return measures.Information{Bits: big.NewFloat(1e9)}, &f, "Gb"
}
