package source

import (
	"fmt"
	"go/ast"
	"io"
)

type StatementSend struct {
	Location
	X     Expression
	Arrow Location
	Value Expression
}

func (pkg *Package) loadStatementSend(in *ast.SendStmt) StatementSend {
	return StatementSend{
		Location: pkg.locations(in.Pos(), in.End()),
		X:        pkg.loadExpression(in.Chan),
		Arrow:    pkg.location(in.Arrow),
		Value:    pkg.loadExpression(in.Value),
	}
}

func (stmt StatementSend) compile(w io.Writer, tabs int) error {
	return fmt.Errorf("send statement not supported")
}
