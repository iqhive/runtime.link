package jit

import (
	"errors"

	"runtime.link/bin"
	"runtime.link/bin/std/cpu/arm64"
	"runtime.link/xyz"
)

func (src *Program) compile() ([]byte, error) {
	bin := bin.NewWriter[arm64.InstructionSet]()
	asm := bin.Encoder
	for _, op := range src.code {
		switch xyz.ValueOf(op) {
		case ops.Add:
			add := ops.Add.Get(op)
			asm.Math.Add(src.gprs[add.dst], src.gprs[add.a], src.gprs[add.b])
		case ops.Mov:
			//TODO
		default:
			return nil, errors.New("jit: unknown op " + xyz.ValueOf(op).String())
		}
	}
	asm.Return()
	return bin.Bytes(), nil
}
