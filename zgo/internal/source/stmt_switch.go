package source

import (
	"go/ast"
	"io"

	"runtime.link/xyz"
)

type StatementSwitchType struct {
	Location
	Keyword Location
	Init    Statement
	Assign  Statement
	Claused []SwitchCaseClause
}

func (pkg *Package) loadStatementSwitchType(in *ast.TypeSwitchStmt) StatementSwitchType {
	var clauses []SwitchCaseClause
	for _, clause := range in.Body.List {
		clauses = append(clauses, pkg.loadSwitchCaseClause(clause.(*ast.CaseClause)))
	}
	return StatementSwitchType{
		Location: pkg.locations(in.Pos(), in.End()),
		Keyword:  pkg.location(in.Switch),
		Init:     pkg.loadStatement(in.Init),
		Assign:   pkg.loadStatement(in.Assign),
		Claused:  clauses,
	}
}

func (stmt StatementSwitchType) compile(w io.Writer, tabs int) error {
	return stmt.Errorf("type-switch statement not supported")
}

type StatementSwitch struct {
	Location
	Keyword Location
	Init    Statement
	Value   xyz.Maybe[Expression]
	Clauses []SwitchCaseClause
}

func (pkg *Package) loadStatementSwitch(in *ast.SwitchStmt) StatementSwitch {
	var value xyz.Maybe[Expression]
	if in.Tag != nil {
		value = xyz.New(pkg.loadExpression(in.Tag))
	}
	var clauses []SwitchCaseClause
	for _, clause := range in.Body.List {
		clauses = append(clauses, pkg.loadSwitchCaseClause(clause.(*ast.CaseClause)))
	}
	return StatementSwitch{
		Location: pkg.locations(in.Pos(), in.End()),
		Keyword:  pkg.location(in.Switch),
		Init:     pkg.loadStatement(in.Init),
		Value:    value,
		Clauses:  clauses,
	}
}

func (stmt StatementSwitch) compile(w io.Writer, tabs int) error {
	return stmt.Errorf("switch statement not supported")
}

type SwitchCaseClause struct {
	Location

	Keyword     Location
	Expressions []Expression
	Colon       Location
	Body        []Statement
}

func (pkg *Package) loadSwitchCaseClause(in *ast.CaseClause) SwitchCaseClause {
	var out SwitchCaseClause
	out.Location = pkg.locations(in.Pos(), in.End())
	out.Keyword = pkg.location(in.Case)
	for _, expr := range in.List {
		out.Expressions = append(out.Expressions, pkg.loadExpression(expr))
	}
	out.Colon = pkg.location(in.Colon)
	for _, stmt := range in.Body {
		out.Body = append(out.Body, pkg.loadStatement(stmt))
	}
	return out
}
