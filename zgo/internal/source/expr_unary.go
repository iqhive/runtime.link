package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
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

func (e ExpressionUnary) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "%s", e.Operation.Value)
	if err := e.X.compile(w, tabs); err != nil {
		return err
	}
	return nil
}
