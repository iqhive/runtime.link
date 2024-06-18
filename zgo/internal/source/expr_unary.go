package source

import (
	"go/ast"
	"go/token"
)

type ExpressionUnary struct {
	typed

	Location

	Operation WithLocation[token.Token]
	X         Expression
}

func (pkg *Package) loadExpressionUnary(in *ast.UnaryExpr) ExpressionUnary {
	return ExpressionUnary{
		Location:  pkg.locations(in.Pos(), in.End()),
		typed:     typed{pkg.Types[in]},
		Operation: WithLocation[token.Token]{Value: in.Op, SourceLocation: pkg.location(in.OpPos)},
		X:         pkg.loadExpression(in.X),
	}
}
