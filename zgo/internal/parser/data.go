package parser

import (
	"go/ast"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func loadDataComposite(pkg *source.Package, in *ast.CompositeLit) source.DataComposite {
	var out source.DataComposite
	out.Location = locationRangeIn(pkg, in, in.Pos(), in.End())
	out.Typed = typedIn(pkg, in)
	if ctype := in.Type; ctype != nil {
		out.Type = xyz.New(loadType(pkg, ctype))
	}
	out.OpenBrace = locationIn(pkg, in, in.Lbrace)
	for _, expr := range in.Elts {
		out.Elements = append(out.Elements, loadExpression(pkg, expr))
	}
	out.CloseBrace = locationIn(pkg, in, in.Rbrace)
	out.Incomplete = in.Incomplete
	return out
}
