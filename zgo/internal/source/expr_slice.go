package source

import (
	"fmt"
	"go/ast"
	"io"

	"runtime.link/xyz"
)

type ExpressionSlice struct {
	typed

	Location

	X        Expression
	Opening  Location
	From     xyz.Maybe[Expression]
	High     xyz.Maybe[Expression]
	Capacity xyz.Maybe[Expression]
	Closing  Location
}

func (pkg *Package) loadExpressionSlice(in *ast.SliceExpr) ExpressionSlice {
	var out ExpressionSlice
	out.Location = pkg.locations(in.Pos(), in.End())
	out.typed = typed{pkg.Types[in]}
	out.X = pkg.loadExpression(in.X)
	out.Opening = pkg.location(in.Lbrack)
	if in.Low != nil {
		out.From = xyz.New(pkg.loadExpression(in.Low))
	}
	if in.High != nil {
		out.High = xyz.New(pkg.loadExpression(in.High))
	}
	if in.Max != nil {
		out.Capacity = xyz.New(pkg.loadExpression(in.Max))
	}
	out.Closing = pkg.location(in.Rbrack)
	return out
}

func (e ExpressionSlice) compile(w io.Writer, tabs int) error {
	if err := e.X.compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, ".range(")
	from, ok := e.From.Get()
	if ok {
		if err := from.compile(w, tabs); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(w, "0")
	}
	fmt.Fprintf(w, ", ")
	high, ok := e.High.Get()
	if ok {
		if err := high.compile(w, tabs); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(w, "0")
	}
	fmt.Fprintf(w, ")")
	return nil
}
