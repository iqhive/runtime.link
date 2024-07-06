package source

import (
	"fmt"
	"go/ast"
	"io"
)

type ExpressionKeyValue struct {
	typed

	Location

	Key   Expression
	Colon Location
	Value Expression
}

func (pkg *Package) loadExpressionKeyValue(in *ast.KeyValueExpr) ExpressionKeyValue {
	return ExpressionKeyValue{
		Location: pkg.locations(in.Pos(), in.End()),
		typed:    pkg.typed(in),
		Key:      pkg.loadExpression(in.Key),
		Colon:    pkg.location(in.Colon),
		Value:    pkg.loadExpression(in.Value),
	}
}

func (e ExpressionKeyValue) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, ".")
	if err := e.Key.compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, "=")
	if err := e.Value.compile(w, tabs); err != nil {
		return err
	}
	return nil
}
