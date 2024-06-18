package source

import "go/ast"

type DataComposite struct {
	typed

	Location

	Type       Type
	OpenBrace  Location
	Elements   []Expression
	CloseBrace Location
	Incomplete bool
}

func (pkg *Package) loadDataComposite(in *ast.CompositeLit) DataComposite {
	var out DataComposite
	out.Location = pkg.locations(in.Pos(), in.End())
	out.typed = typed{pkg.Types[in]}
	out.Type = pkg.loadType(in.Type)
	out.OpenBrace = pkg.location(in.Lbrace)
	for _, expr := range in.Elts {
		out.Elements = append(out.Elements, pkg.loadExpression(expr))
	}
	out.CloseBrace = pkg.location(in.Rbrace)
	out.Incomplete = in.Incomplete
	return out
}
