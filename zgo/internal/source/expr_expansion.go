package source

import "go/ast"

type ExpressionExpansion struct {
	typed

	Location

	Expression WithLocation[Expression]
}

func (pkg *Package) loadExpressionExpansion(in *ast.Ellipsis) ExpressionExpansion {
	return ExpressionExpansion{
		Location: pkg.locations(in.Pos(), in.End()),
		typed:    typed{pkg.Types[in]},
		Expression: WithLocation[Expression]{
			Value:          pkg.loadExpression(in.Elt),
			SourceLocation: pkg.location(in.Ellipsis),
		},
	}
}
