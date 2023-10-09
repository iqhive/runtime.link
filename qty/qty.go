// Package qty provides a standard way to represent quantities with different underlying units.
package qty

import (
	"math/big"
)

type Time interface {
	Seconds() *big.Float
}

type Length interface {
	Metres() *big.Float
}

type Mass interface {
	Grams() *big.Float
}

type Current interface {
	Amps() *big.Float
}

type Temperature interface {
	Kelvin() *big.Float
}
