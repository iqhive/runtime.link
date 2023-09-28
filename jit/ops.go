package jit

import (
	"runtime.link/xyz"
)

type op xyz.Switch[[4]int, struct {
	Add xyz.Case[op, opAdd]
	Mov xyz.Case[op, opMov]
}]

var ops = xyz.AccessorFor(op.Values)

type opAdd struct {
	dst, a, b gprIndex
}

type opMov struct {
	dst, src gprIndex
}
