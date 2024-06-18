package source

import "go/ast"

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
		typed:    typed{pkg.Types[in]},
		Key:      pkg.loadExpression(in.Key),
		Colon:    pkg.location(in.Colon),
		Value:    pkg.loadExpression(in.Value),
	}
}
