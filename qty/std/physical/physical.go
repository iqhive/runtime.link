// Package physical provides a standard way to represent physical quantities using SI units.
package physical

import "math/big"

// Mass in grams.
type Mass struct {
	Grams *big.Float
}

func (mass Mass) Float() *big.Float { return mass.Grams }

func (mass Mass) As(grams *big.Float) Mass { return Mass{Grams: grams} }

// Distance in metres.
type Distance struct {
	Metres *big.Float
}

func (distance Distance) Float() *big.Float             { return distance.Metres }
func (distance Distance) As(metres *big.Float) Distance { return Distance{Metres: metres} }

// Duration in seconds.
type Duration struct {
	Seconds *big.Float
}

func (duration Duration) Float() *big.Float              { return duration.Seconds }
func (duration Duration) As(seconds *big.Float) Duration { return Duration{Seconds: seconds} }

// Current in amps.
type Current struct {
	Amps *big.Float
}

func (current Current) Float() *big.Float          { return current.Amps }
func (current Current) As(amps *big.Float) Current { return Current{Amps: amps} }

// Temperature in kelvin.
type Temperature struct {
	Kelvin *big.Float
}

func (temperature Temperature) Float() *big.Float                { return temperature.Kelvin }
func (temperature Temperature) As(kelvin *big.Float) Temperature { return Temperature{Kelvin: kelvin} }

// Substance in moles.
type Substance struct {
	Moles *big.Float
}

func (substance Substance) Float() *big.Float             { return substance.Moles }
func (substance Substance) As(moles *big.Float) Substance { return Substance{Moles: moles} }

// Brightness in candela.
type Brightness struct {
	Candelas *big.Float
}

func (brightness Brightness) Float() *big.Float                { return brightness.Candelas }
func (brightness Brightness) As(candela *big.Float) Brightness { return Brightness{Candelas: candela} }

// Information in bits.
type Information struct {
	Bits *big.Float
}

func (information Information) Float() *big.Float              { return information.Bits }
func (information Information) As(bits *big.Float) Information { return Information{Bits: bits} }
