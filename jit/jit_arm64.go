package jit

import (
	"runtime.link/cpu"
	"runtime.link/cpu/std/arm64"
	"runtime.link/xyz"
)

func (src *Program) compile() []byte {
	bin := cpu.NewProgram[arm64.InstructionSet]()
	asm := bin.Assembly
	for _, op := range src.code {
		switch xyz.ValueOf(op) {
		case ops.Add:
			add := ops.Add.Get(op)
			asm.Math.Add(src.gprs[add.dst], src.gprs[add.a], src.gprs[add.b])
		case ops.Mov:
			//TODO
		default:
			panic("jit: unknown op " + xyz.ValueOf(op).String())
		}
	}
	asm.Return()
	return bin.Bytes()
}
