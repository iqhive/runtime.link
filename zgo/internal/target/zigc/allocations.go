package zigc

import "runtime.link/zgo/internal/source"

func (zig Target) StackAllocated(ident source.DefinedVariable) bool {
	if ident.Escapes.Block == nil {
		return true
	}
	return ident.Package || (!ident.Escapes.Function().Possible && !ident.Escapes.Block().Possible && !ident.Escapes.Goroutine().Possible && !ident.Escapes.Containment().Possible)
}
