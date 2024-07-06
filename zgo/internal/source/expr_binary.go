package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
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
		typed:     pkg.typed(in),
		X:         pkg.loadExpression(in.X),
		Operation: WithLocation[token.Token]{Value: in.Op, SourceLocation: pkg.location(in.OpPos)},
		Y:         pkg.loadExpression(in.Y),
	}
}

func (expr ExpressionBinary) compile(w io.Writer, tabs int) error {
	switch expr.Operation.Value {
	case token.NEQ:
		switch etype := expr.X.TypeAndValue().Type.(type) {
		case *types.Basic:
			switch etype.Kind() {
			case types.String, types.UntypedString:
				fmt.Fprintf(w, "(!go.equality(%s, %s,%s))", expr.X.ZigType(), toString(expr.X), toString(expr.Y))
				return nil
			}
		}
	}
	if err := expr.X.compile(w, tabs); err != nil {
		return err
	}
	switch expr.Operation.Value {
	case token.LOR:
		fmt.Fprintf(w, " or ")
	case token.LAND:
		fmt.Fprintf(w, " and ")
	default:
		fmt.Fprintf(w, " %s ", expr.Operation.Value)
	}
	return expr.Y.compile(w, tabs)
}
