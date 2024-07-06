package source

import (
	"fmt"
	"go/ast"
	"io"
)

type ExpressionReceive struct {
	typed

	Location

	Chan Expression
}

func (pkg *Package) loadExpressionReceive(in *ast.UnaryExpr) ExpressionReceive {
	return ExpressionReceive{
		Location: pkg.locations(in.Pos(), in.End()),
		typed:    pkg.typed(in),
		Chan:     pkg.loadExpression(in.X),
	}
}

func (e ExpressionReceive) compile(w io.Writer, tabs int) error {
	if err := e.Chan.compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprint(w, ".recv(goto)")
	return nil
}
