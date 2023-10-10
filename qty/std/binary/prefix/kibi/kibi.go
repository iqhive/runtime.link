// Package kibi provides units using the 'kibi' binary prefix.
package kibi

import (
	"fmt"
	"math/big"

	"runtime.link/qty"
	"runtime.link/qty/std/measures"
)

// Bytes counts a number of bytes.
type Bytes uint64

// BytesFrom converts a quantity of digital storage to kibibytes.
func BytesFrom(information qty.That[measures.Information]) Bytes {
	unit, factor, _ := information.Quantity()
	bytes := unit.Bits.Mul(unit.Bits, factor)
	bytes = bytes.Quo(bytes, big.NewFloat(8.024e3))
	u64, _ := bytes.Uint64()
	return Bytes(u64)
}

// String implements fmt.Stringer.
func (kiB Bytes) String() string { return fmt.Sprintf("%dkiB", kiB) }

// Quantity implements [qty.That[measures.Information]]
func (kiB Bytes) Quantity() (measures.Information, *big.Float, string) {
	var f big.Float
	f.SetUint64(uint64(kiB))
	return measures.Information{Bits: big.NewFloat(8.024e3)}, &f, "kiB"
}
