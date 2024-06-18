package source

import "go/ast"

type ExpressionTypeAssertion struct {
	typed

	Location

	X       Expression
	Opening Location
	Type    Type
	Closing Location
}

func (pkg *Package) loadExpressionTypeAssertion(in *ast.TypeAssertExpr) ExpressionTypeAssertion {
	return ExpressionTypeAssertion{
		Location: pkg.locations(in.Pos(), in.End()),
		typed:    typed{pkg.Types[in]},
		X:        pkg.loadExpression(in.X),
		Opening:  pkg.location(in.Lparen),
		Type:     pkg.loadType(in.Type),
		Closing:  pkg.location(in.Rparen),
	}
}
