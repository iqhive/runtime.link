package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
)

type ExpressionBinary struct {
	typed

	X         Expression
	Operation WithLocation[token.Token]
	Y         Expression
}

func (pkg *Package) loadExpressionBinary(in *ast.BinaryExpr) ExpressionBinary {
	return ExpressionBinary{
		typed:     typed{pkg.Types[in]},
		X:         pkg.loadExpression(in.X),
		Operation: WithLocation[token.Token]{Value: in.Op, SourceLocation: Location(in.OpPos)},
		Y:         pkg.loadExpression(in.Y),
	}
}

func (expr ExpressionBinary) compile(w io.Writer) error {
	if err := expr.X.compile(w); err != nil {
		return err
	}
	fmt.Fprintf(w, " %s ", expr.Operation.Value)
	return expr.Y.compile(w)
}
