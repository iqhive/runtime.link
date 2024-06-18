package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
)

type ExpressionBinary struct {
	Location

	typed

	X         Expression
	Operation WithLocation[token.Token]
	Y         Expression
}

func (pkg *Package) loadExpressionBinary(in *ast.BinaryExpr) ExpressionBinary {
	return ExpressionBinary{
		Location:  pkg.locations(in.Pos(), in.End()),
		typed:     typed{pkg.Types[in]},
		X:         pkg.loadExpression(in.X),
		Operation: WithLocation[token.Token]{Value: in.Op, SourceLocation: pkg.location(in.OpPos)},
		Y:         pkg.loadExpression(in.Y),
	}
}

func (expr ExpressionBinary) compile(w io.Writer, tabs int) error {
	if err := expr.X.compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, " %s ", expr.Operation.Value)
	return expr.Y.compile(w, tabs)
}
