package source

import (
	"fmt"
	"go/ast"
	"io"
)

type StatementGo struct {
	Location
	Keyword Location
	Call    ExpressionCall
}

func (pkg *Package) loadStatementGo(in *ast.GoStmt) StatementGo {
	return StatementGo{
		Location: pkg.locations(in.Pos(), in.End()),
		Keyword:  pkg.location(in.Go),
		Call:     pkg.loadExpressionCall(in.Call),
	}
}

func (stmt StatementGo) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "go ")
	return stmt.Call.compile(w, tabs)
}
