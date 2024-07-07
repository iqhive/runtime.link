package source

import (
	"go/ast"
	"io"

	"runtime.link/xyz"
)

type ExpressionExpansion struct {
	typed

	Location

	Expression xyz.Maybe[Expression]
}

func (pkg *Package) loadExpressionExpansion(in *ast.Ellipsis) ExpressionExpansion {
	var expression xyz.Maybe[Expression]
	if in.Elt != nil {
		expression = xyz.New(pkg.loadExpression(in.Elt))
	}
	return ExpressionExpansion{
		Location:   pkg.locations(in.Pos(), in.End()),
		typed:      pkg.typed(in),
		Expression: expression,
	}
}

func (exp ExpressionExpansion) compile(w io.Writer, tabs int) error {

	return exp.Errorf("expression expansion not supported")
}
