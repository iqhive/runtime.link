package measures

import "math/big"

// Information in bits.
type Information struct{ Bits *big.Float }

func (information Information) Float() *big.Float              { return information.Bits }
func (information Information) As(bits *big.Float) Information { return Information{Bits: bits} }

// Resolution in pixels.
type Resolution struct{ Pixels *big.Float }

func (resolution Resolution) Float() *big.Float               { return resolution.Pixels }
func (resolution Resolution) As(pixels *big.Float) Resolution { return Resolution{Pixels: pixels} }
