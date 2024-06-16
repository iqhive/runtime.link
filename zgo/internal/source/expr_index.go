package source

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"
)

type ExpressionIndex struct {
	typed

	X       Expression
	Opening Location
	Index   Expression
	Closing Location
}

func (pkg *Package) loadExpressionIndex(in *ast.IndexExpr) ExpressionIndex {
	return ExpressionIndex{
		typed:   typed{pkg.Types[in]},
		X:       pkg.loadExpression(in.X),
		Opening: Location(in.Lbrack),
		Index:   pkg.loadExpression(in.Index),
		Closing: Location(in.Rbrack),
	}
}

func (expr ExpressionIndex) compile(w io.Writer) error {
	switch expr.X.TypeAndValue().Type.(type) {
	case *types.Slice:
		if err := expr.X.compile(w); err != nil {
			return err
		}
		fmt.Fprintf(w, ".items[")
		if err := expr.Index.compile(w); err != nil {
			return err
		}
		fmt.Fprintf(w, "]")
		return nil
	case *types.Map:
		if err := expr.X.compile(w); err != nil {
			return err
		}
		fmt.Fprintf(w, ".get(")
		if err := expr.Index.compile(w); err != nil {
			return err
		}
		fmt.Fprintf(w, ")")
		return nil
	default:
		return fmt.Errorf("unsupported index of type %T", expr)
	}
}
