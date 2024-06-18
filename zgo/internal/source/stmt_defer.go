package source

import (
	"fmt"
	"go/ast"
	"io"
)

type StatementDefer struct {
	Location

	Keyword Location
	Call    ExpressionCall

	OutermostScope bool
}

func (pkg *Package) loadStatementDefer(in *ast.DeferStmt) StatementDefer {
	return StatementDefer{
		Location: pkg.locations(in.Pos(), in.End()),
		Keyword:  pkg.location(in.Defer),
		Call:     pkg.loadExpressionCall(in.Call),
	}
}

func (stmt StatementDefer) compile(w io.Writer, tabs int) error {
	// TODO arguments need to be evaluated at the time of the defer statement.
	if stmt.OutermostScope {
		fmt.Fprintf(w, "defer ")
		return stmt.Call.compile(w, tabs)
	}
	return stmt.Location.Errorf("only defer at the outermost scope of a function is currently supported")
}
