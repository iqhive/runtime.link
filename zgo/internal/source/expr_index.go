package source

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"
)

type ExpressionIndex struct {
	Location

	typed

	X       Expression
	Opening Location
	Index   Expression
	Closing Location
}

func (pkg *Package) loadExpressionIndex(in *ast.IndexExpr) ExpressionIndex {
	return ExpressionIndex{
		Location: pkg.locations(in.Pos(), in.End()),
		typed:    pkg.typed(in),
		X:        pkg.loadExpression(in.X),
		Opening:  pkg.location(in.Lbrack),
		Index:    pkg.loadExpression(in.Index),
		Closing:  pkg.location(in.Rbrack),
	}
}

func (expr ExpressionIndex) compile(w io.Writer, tabs int) error {
	switch expr.X.TypeAndValue().Type.(type) {
	case *types.Slice:
		if err := expr.X.compile(w, tabs); err != nil {
			return err
		}
		fmt.Fprintf(w, ".index(")
		if err := expr.Index.compile(w, tabs); err != nil {
			return err
		}
		fmt.Fprintf(w, ")")
		return nil
	case *types.Map:
		if err := expr.X.compile(w, tabs); err != nil {
			return err
		}
		fmt.Fprintf(w, ".get(")
		if err := expr.Index.compile(w, tabs); err != nil {
			return err
		}
		fmt.Fprintf(w, ")")
		return nil
	case *types.Array:
		if err := expr.X.compile(w, tabs); err != nil {
			return err
		}
		fmt.Fprintf(w, "[")
		if err := expr.Index.compile(w, tabs); err != nil {
			return err
		}
		fmt.Fprintf(w, "]")
		return nil
	default:
		return fmt.Errorf("unsupported index of type %T", expr)
	}
}
