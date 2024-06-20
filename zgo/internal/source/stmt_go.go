package source

import (
	"go/ast"
	"io"
)

type StatementGo struct {
	Location
	Keyword Location
	Call    ExpressionCall
}

func (pkg *Package) loadStatementGo(in *ast.GoStmt) StatementGo {
	call := pkg.loadExpressionCall(in.Call)
	call.Go = true
	return StatementGo{
		Location: pkg.locations(in.Pos(), in.End()),
		Keyword:  pkg.location(in.Go),
		Call:     call,
	}
}

func (stmt StatementGo) compile(w io.Writer, tabs int) error {
	return stmt.Call.compile(w, tabs)
}
