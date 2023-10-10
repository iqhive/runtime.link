// Package mebi provides a standard way to represent 1048576's of bytes.
package mebi

import (
	"fmt"
	"math/big"

	"runtime.link/qty"
	"runtime.link/qty/std/physical"
)

// Bytes counts a number of bytes.
type Bytes uint64

// BytesFrom converts a quantity of digital storage to mebibytes.
func BytesFrom(information qty.Measures[physical.Information]) Bytes {
	unit, factor, _ := information.Quantity()
	bytes := unit.Bits.Mul(unit.Bits, factor)
	bytes = bytes.Quo(bytes, big.NewFloat(8*1024*1024))
	u64, _ := bytes.Uint64()
	return Bytes(u64)
}

// String implements fmt.Stringer.
func (MiB Bytes) String() string { return fmt.Sprintf("%dMiB", MiB) }

// Quantity implements [qty.Measures[physical.Information]]
func (MiB Bytes) Quantity() (physical.Information, *big.Float, string) {
	var f big.Float
	f.SetUint64(uint64(MiB))
	return physical.Information{Bits: big.NewFloat(8 * 1024 * 1024)}, &f, "MiB"
}
