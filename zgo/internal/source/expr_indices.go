package source

import "go/ast"

type ExpressionIndices struct {
	Location

	typed

	X        Expression
	Opening  Location
	Indicies []Expression
	Closing  Location
}

func (pkg *Package) loadExpressionIndices(in *ast.IndexListExpr) ExpressionIndices {
	var out ExpressionIndices
	out.Location = pkg.locations(in.Pos(), in.End())
	out.typed = pkg.typed(in)
	out.X = pkg.loadExpression(in.X)
	out.Opening = pkg.location(in.Lbrack)
	for _, index := range in.Indices {
		out.Indicies = append(out.Indicies, pkg.loadExpression(index))
	}
	out.Closing = pkg.location(in.Rbrack)
	return out
}
