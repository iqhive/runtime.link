package source

import (
	"fmt"
	"go/ast"
	"io"
	"strings"
)

type StatementBlock struct {
	Location

	Opening    Location
	Statements []Statement
	Closing    Location
}

func (pkg *Package) loadStatementBlock(in *ast.BlockStmt) StatementBlock {
	var out StatementBlock
	out.Location = pkg.locations(in.Pos(), in.End())
	out.Opening = pkg.location(in.Lbrace)
	for _, stmt := range in.List {
		out.Statements = append(out.Statements, pkg.loadStatement(stmt))
	}
	out.Closing = pkg.location(in.Rbrace)
	return out
}

func (stmt StatementBlock) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "{")
	for _, stmt := range stmt.Statements {
		if err := stmt.compile(w, tabs+1); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
	fmt.Fprintf(w, "}")
	return nil
}
