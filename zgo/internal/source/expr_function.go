package source

import "go/ast"

type ExpressionFunction struct {
	typed

	Location

	Type TypeFunction
	Body StatementBlock
}

func (pkg *Package) loadExpressionFunction(in *ast.FuncLit) ExpressionFunction {
	var out ExpressionFunction
	out.Location = pkg.locations(in.Pos(), in.End())
	out.typed = typed{pkg.Types[in]}
	out.Type = pkg.loadTypeFunction(in.Type)
	out.Body = pkg.loadStatementBlock(in.Body)
	return out
}
