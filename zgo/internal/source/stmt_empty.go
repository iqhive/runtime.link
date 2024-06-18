package source

import (
	"go/ast"
	"io"
)

type StatementEmpty struct {
	Location
	Semicolon Location
	Implicit  bool
}

func (pkg *Package) loadStatementEmpty(in *ast.EmptyStmt) StatementEmpty {
	return StatementEmpty{
		Location:  pkg.locations(in.Pos(), in.End()),
		Semicolon: pkg.location(in.Semicolon),
		Implicit:  in.Implicit,
	}
}

func (stmt StatementEmpty) compile(w io.Writer, tabs int) error {
	return nil
}
