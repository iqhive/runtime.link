package cpu

// callFast runs a program with only the registers of [RegistersFast].
func (p *Program) callFast(reg RegistersFast) RegistersFast {
	var full RegistersArch
	*full.R0(), *full.R1(), *full.X0() = *reg.R0(), *reg.R1(), *reg.X0()
	full = p.callArch(full)
	*reg.R0(), *reg.R1(), *reg.X0() = *full.R0(), *full.R1(), *full.X0()
	return reg
}
