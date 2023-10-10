package measures

import "math/big"

// Area in square metres.
type Area struct{ SquareMetres *big.Float }

func (area Area) Float() *big.Float               { return area.SquareMetres }
func (area Area) As(squareMetres *big.Float) Area { return Area{SquareMetres: squareMetres} }

// Volume in cubic metres.
type Volume struct{ CubicMetres *big.Float }

func (volume Volume) Float() *big.Float                { return volume.CubicMetres }
func (volume Volume) As(cubicMetres *big.Float) Volume { return Volume{CubicMetres: cubicMetres} }

// Velocity in metres per second.
type Velocity struct{ MetresPerSecond *big.Float }

func (velocity Velocity) Float() *big.Float { return velocity.MetresPerSecond }
func (velocity Velocity) As(metresPerSecond *big.Float) Velocity {
	return Velocity{MetresPerSecond: metresPerSecond}
}

// Acceleration in metres per second squared.
type Acceleration struct{ MetresPerSecondSquared *big.Float }

func (acceleration Acceleration) Float() *big.Float { return acceleration.MetresPerSecondSquared }
func (acceleration Acceleration) As(metresPerSecondSquared *big.Float) Acceleration {
	return Acceleration{MetresPerSecondSquared: metresPerSecondSquared}
}

// VolumeDensity in kilograms per cubic metre.
type VolumeDensity struct{ KilogramsPerCubicMetre *big.Float }

func (density VolumeDensity) Float() *big.Float { return density.KilogramsPerCubicMetre }
func (density VolumeDensity) As(kilogramsPerCubicMetre *big.Float) VolumeDensity {
	return VolumeDensity{KilogramsPerCubicMetre: kilogramsPerCubicMetre}
}

// SurfaceDensity in kilograms per square metre.
type SurfaceDensity struct{ KilogramsPerSquareMetre *big.Float }

func (surfaceDensity SurfaceDensity) Float() *big.Float {
	return surfaceDensity.KilogramsPerSquareMetre
}
func (surfaceDensity SurfaceDensity) As(kilogramsPerSquareMetre *big.Float) SurfaceDensity {
	return SurfaceDensity{KilogramsPerSquareMetre: kilogramsPerSquareMetre}
}

// SpecificVolume in cubic metres per kilogram.
type SpecificVolume struct{ CubicMetresPerKilogram *big.Float }

func (specificVolume SpecificVolume) Float() *big.Float { return specificVolume.CubicMetresPerKilogram }
func (specificVolume SpecificVolume) As(cubicMetresPerKilogram *big.Float) SpecificVolume {
	return SpecificVolume{CubicMetresPerKilogram: cubicMetresPerKilogram}
}

// CurrentDensity in amps per square metre.
type CurrentDensity struct{ AmpsPerSquareMetre *big.Float }

func (currentDensity CurrentDensity) Float() *big.Float { return currentDensity.AmpsPerSquareMetre }
func (currentDensity CurrentDensity) As(ampsPerSquareMetre *big.Float) CurrentDensity {
	return CurrentDensity{AmpsPerSquareMetre: ampsPerSquareMetre}
}

// MagneticFieldStrength in amps per metre.
type MagneticFieldStrength struct{ AmpsPerMetre *big.Float }

func (magneticFieldStrength MagneticFieldStrength) Float() *big.Float {
	return magneticFieldStrength.AmpsPerMetre
}
func (magneticFieldStrength MagneticFieldStrength) As(ampsPerMetre *big.Float) MagneticFieldStrength {
	return MagneticFieldStrength{AmpsPerMetre: ampsPerMetre}
}

// ChemicalConcentration in moles per cubic metre.
type ChemicalConcentration struct{ MolesPerCubicMetre *big.Float }

func (chemicalConcentration ChemicalConcentration) Float() *big.Float {
	return chemicalConcentration.MolesPerCubicMetre
}
func (chemicalConcentration ChemicalConcentration) As(molesPerCubicMetre *big.Float) ChemicalConcentration {
	return ChemicalConcentration{MolesPerCubicMetre: molesPerCubicMetre}
}

// MassConcentration in kilograms per cubic metre.
type MassConcentration struct{ KilogramsPerCubicMetre *big.Float }

func (massConcentration MassConcentration) Float() *big.Float {
	return massConcentration.KilogramsPerCubicMetre
}
func (massConcentration MassConcentration) As(kilogramsPerCubicMetre *big.Float) MassConcentration {
	return MassConcentration{KilogramsPerCubicMetre: kilogramsPerCubicMetre}
}

// Luminance in candelas per square metre.
type Luminance struct{ CandelasPerSquareMetre *big.Float }

func (luminance Luminance) Float() *big.Float { return luminance.CandelasPerSquareMetre }
func (luminance Luminance) As(candelasPerSquareMetre *big.Float) Luminance {
	return Luminance{CandelasPerSquareMetre: candelasPerSquareMetre}
}

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

// RadiationAbsorbedDose in grays.
type RadiationAbsorbedDose struct{ Grays *big.Float }

func (dose RadiationAbsorbedDose) Float() *big.Float { return dose.Grays }
func (dose RadiationAbsorbedDose) As(grays *big.Float) RadiationAbsorbedDose {
	return RadiationAbsorbedDose{Grays: grays}
}

// RadiationEquivalentDose in sieverts.
type RadiationEquivalentDose struct{ Sieverts *big.Float }

func (dose RadiationEquivalentDose) Float() *big.Float { return dose.Sieverts }
func (dose RadiationEquivalentDose) As(sieverts *big.Float) RadiationEquivalentDose {
	return RadiationEquivalentDose{Sieverts: sieverts}
}

// Catalysis in katal.
type Catalysis struct{ Katals *big.Float }

func (catalysis Catalysis) Float() *big.Float { return catalysis.Katals }
func (catalysis Catalysis) As(katals *big.Float) Catalysis {
	return Catalysis{Katals: katals}
}
