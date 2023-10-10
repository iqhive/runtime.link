// Package measures provides a standard measures.
package measures

import "math/big"

// Mass in grams.
type Mass struct{ Grams *big.Float }

func (mass Mass) Float() *big.Float        { return mass.Grams }
func (mass Mass) As(grams *big.Float) Mass { return Mass{Grams: grams} }

// Distance in metres.
type Distance struct{ Metres *big.Float }

func (distance Distance) Float() *big.Float             { return distance.Metres }
func (distance Distance) As(metres *big.Float) Distance { return Distance{Metres: metres} }

// Duration in seconds.
type Duration struct{ Seconds *big.Float }

func (duration Duration) Float() *big.Float              { return duration.Seconds }
func (duration Duration) As(seconds *big.Float) Duration { return Duration{Seconds: seconds} }

// Current in amps.
type Current struct{ Amps *big.Float }

func (current Current) Float() *big.Float          { return current.Amps }
func (current Current) As(amps *big.Float) Current { return Current{Amps: amps} }

// Temperature in kelvin.
type Temperature struct{ Kelvin *big.Float }

func (temperature Temperature) Float() *big.Float                { return temperature.Kelvin }
func (temperature Temperature) As(kelvin *big.Float) Temperature { return Temperature{Kelvin: kelvin} }

// Substance in moles.
type Substance struct{ Moles *big.Float }

func (substance Substance) Float() *big.Float             { return substance.Moles }
func (substance Substance) As(moles *big.Float) Substance { return Substance{Moles: moles} }

// Brightness in candela.
type Brightness struct{ Candelas *big.Float }

func (brightness Brightness) Float() *big.Float                { return brightness.Candelas }
func (brightness Brightness) As(candela *big.Float) Brightness { return Brightness{Candelas: candela} }

// Information in bits.
type Information struct{ Bits *big.Float }

func (information Information) Float() *big.Float              { return information.Bits }
func (information Information) As(bits *big.Float) Information { return Information{Bits: bits} }

// Resolution in pixels.
type Resolution struct{ Pixels *big.Float }

func (resolution Resolution) Float() *big.Float               { return resolution.Pixels }
func (resolution Resolution) As(pixels *big.Float) Resolution { return Resolution{Pixels: pixels} }

// Frequency in hertz.
type Frequency struct{ Hertz *big.Float }

func (frequency Frequency) Float() *big.Float             { return frequency.Hertz }
func (frequency Frequency) As(hertz *big.Float) Frequency { return Frequency{Hertz: hertz} }

// Force in newtons.
type Force struct{ Newtons *big.Float }

func (force Force) Float() *big.Float           { return force.Newtons }
func (force Force) As(newtons *big.Float) Force { return Force{Newtons: newtons} }

// Pressure in pascals.
type Pressure struct{ Pascals *big.Float }

func (pressure Pressure) Float() *big.Float              { return pressure.Pascals }
func (pressure Pressure) As(pascals *big.Float) Pressure { return Pressure{Pascals: pascals} }

// Energy in joules.
type Energy struct{ Joules *big.Float }

func (energy Energy) Float() *big.Float           { return energy.Joules }
func (energy Energy) As(joules *big.Float) Energy { return Energy{Joules: joules} }

// Power in watts.
type Power struct{ Watts *big.Float }

func (power Power) Float() *big.Float         { return power.Watts }
func (power Power) As(watts *big.Float) Power { return Power{Watts: watts} }

// Charge in coulombs.
type Charge struct{ Coulombs *big.Float }

func (charge Charge) Float() *big.Float             { return charge.Coulombs }
func (charge Charge) As(coulombs *big.Float) Charge { return Charge{Coulombs: coulombs} }

// Voltage in volts.
type Voltage struct{ Volts *big.Float }

func (voltage Voltage) Float() *big.Float           { return voltage.Volts }
func (voltage Voltage) As(volts *big.Float) Voltage { return Voltage{Volts: volts} }

// Capacitance in farads.
type Capacitance struct{ Farads *big.Float }

func (capacitance Capacitance) Float() *big.Float                { return capacitance.Farads }
func (capacitance Capacitance) As(farads *big.Float) Capacitance { return Capacitance{Farads: farads} }

// Resistance in ohms.
type Resistance struct{ Ohms *big.Float }

func (resistance Resistance) Float() *big.Float             { return resistance.Ohms }
func (resistance Resistance) As(ohms *big.Float) Resistance { return Resistance{Ohms: ohms} }

// Conductance in siemens.
type Conductance struct{ Siemens *big.Float }

func (conductance Conductance) Float() *big.Float { return conductance.Siemens }
func (conductance Conductance) As(siemens *big.Float) Conductance {
	return Conductance{Siemens: siemens}
}

// MagneticFlux in webers.
type MagneticFlux struct{ Webers *big.Float }

func (flux MagneticFlux) Float() *big.Float                 { return flux.Webers }
func (flux MagneticFlux) As(webers *big.Float) MagneticFlux { return MagneticFlux{Webers: webers} }

// MagneticFluxDensity in teslas.
type MagneticFluxDensity struct{ Teslas *big.Float }

func (fluxDensity MagneticFluxDensity) Float() *big.Float { return fluxDensity.Teslas }
func (fluxDensity MagneticFluxDensity) As(teslas *big.Float) MagneticFluxDensity {
	return MagneticFluxDensity{Teslas: teslas}
}

// Inductance in henrys.
type Inductance struct{ Henrys *big.Float }

func (inductance Inductance) Float() *big.Float               { return inductance.Henrys }
func (inductance Inductance) As(henrys *big.Float) Inductance { return Inductance{Henrys: henrys} }

// LuminousFlux in lumens.
type LuminousFlux struct{ Lumens *big.Float }

func (flux LuminousFlux) Float() *big.Float                 { return flux.Lumens }
func (flux LuminousFlux) As(lumens *big.Float) LuminousFlux { return LuminousFlux{Lumens: lumens} }

// Illuminance in lux.
type Illuminance struct{ Lux *big.Float }

func (illuminance Illuminance) Float() *big.Float             { return illuminance.Lux }
func (illuminance Illuminance) As(lux *big.Float) Illuminance { return Illuminance{Lux: lux} }

// Radioactivity in becquerels.
type Radioactivity struct{ Becquerels *big.Float }

func (radioactivity Radioactivity) Float() *big.Float { return radioactivity.Becquerels }
func (radioactivity Radioactivity) As(becquerels *big.Float) Radioactivity {
	return Radioactivity{Becquerels: becquerels}
}

// RadioabsorbedDose in grays.
type RadioabsorbedDose struct{ Grays *big.Float }

func (dose RadioabsorbedDose) Float() *big.Float { return dose.Grays }
func (dose RadioabsorbedDose) As(grays *big.Float) RadioabsorbedDose {
	return RadioabsorbedDose{Grays: grays}
}

// RadioequivalentDose in sieverts.
type RadioequivalentDose struct{ Sieverts *big.Float }

func (dose RadioequivalentDose) Float() *big.Float { return dose.Sieverts }
func (dose RadioequivalentDose) As(sieverts *big.Float) RadioequivalentDose {
	return RadioequivalentDose{Sieverts: sieverts}
}

// Catalysis in katal.
type Catalysis struct{ Katals *big.Float }

func (catalysis Catalysis) Float() *big.Float { return catalysis.Katals }
func (catalysis Catalysis) As(katals *big.Float) Catalysis {
	return Catalysis{Katals: katals}
}
