package source

import (
	"go/ast"
	"io"
)

type StatementSelect struct {
	Location
	Keyword Location
	Clauses []SelectCaseClause
}

func (pkg *Package) loadStatementSelect(in *ast.SelectStmt) StatementSelect {
	var clauses []SelectCaseClause
	for _, clause := range in.Body.List {
		clauses = append(clauses, pkg.loadSelectCaseClause(clause.(*ast.CommClause)))
	}
	return StatementSelect{
		Location: pkg.locations(in.Pos(), in.End()),
		Keyword:  pkg.location(in.Select),
		Clauses:  clauses,
	}
}

func (stmt StatementSelect) compile(w io.Writer, tabs int) error {
	return stmt.Errorf("select statement not supported")
}

type SelectCaseClause struct {
	Location

	Keyword   Location
	Statement Statement
	Colon     Location
	Body      []Statement
}

func (pkg *Package) loadSelectCaseClause(in *ast.CommClause) SelectCaseClause {
	var out SelectCaseClause
	out.Location = pkg.locations(in.Pos(), in.End())
	out.Keyword = pkg.location(in.Case)
	out.Statement = pkg.loadStatement(in.Comm)
	out.Colon = pkg.location(in.Colon)
	for _, stmt := range in.Body {
		out.Body = append(out.Body, pkg.loadStatement(stmt))
	}
	return out
}
