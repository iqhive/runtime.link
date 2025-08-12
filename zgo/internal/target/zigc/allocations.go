package zigc

import "runtime.link/zgo/internal/source"

func (zig Target) StackAllocated(ident source.DefinedVariable) bool {
	return !ident.Escapes.Function() && !ident.Escapes.Block() && !ident.Escapes.Goroutine() && !ident.Escapes.Containment()
}
