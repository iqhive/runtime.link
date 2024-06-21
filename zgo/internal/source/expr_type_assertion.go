package source

import (
	"fmt"
	"go/ast"
	"io"

	"runtime.link/xyz"
)

type ExpressionTypeAssertion struct {
	typed

	Location

	X       Expression
	Opening Location
	Type    xyz.Maybe[Type]
	Closing Location
}

func (pkg *Package) loadExpressionTypeAssertion(in *ast.TypeAssertExpr) ExpressionTypeAssertion {
	var stype xyz.Maybe[Type]
	if in.Type != nil {
		stype = xyz.New(pkg.loadType(in.Type))
	}
	return ExpressionTypeAssertion{
		Location: pkg.locations(in.Pos(), in.End()),
		typed:    typed{pkg.Types[in]},
		X:        pkg.loadExpression(in.X),
		Opening:  pkg.location(in.Lparen),
		Type:     stype,
		Closing:  pkg.location(in.Rparen),
	}
}

func (e ExpressionTypeAssertion) compile(w io.Writer, tabs int) error {
	if err := e.X.compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, " .(%s)", e.Type)
	return nil
}
