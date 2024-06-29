package source

import (
	"fmt"
	"go/ast"
	"io"
	"strings"

	"runtime.link/xyz"
)

type StatementSwitchType struct {
	Location
	Keyword Location
	Init    xyz.Maybe[Statement]
	Assign  Statement
	Claused []SwitchCaseClause
}

func (pkg *Package) loadStatementSwitchType(in *ast.TypeSwitchStmt) StatementSwitchType {
	var clauses []SwitchCaseClause
	for _, clause := range in.Body.List {
		clauses = append(clauses, pkg.loadSwitchCaseClause(clause.(*ast.CaseClause)))
	}
	var init xyz.Maybe[Statement]
	if in.Init != nil {
		init = xyz.New(pkg.loadStatement(in.Init))
	}
	return StatementSwitchType{
		Location: pkg.locations(in.Pos(), in.End()),
		Keyword:  pkg.location(in.Switch),
		Init:     init,
		Assign:   pkg.loadStatement(in.Assign),
		Claused:  clauses,
	}
}

func (stmt StatementSwitchType) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "switch ")
	if init, ok := stmt.Init.Get(); ok {
		if err := init.compile(w, tabs); err != nil {
			return err
		}
	}
	if err := stmt.Assign.compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, " {")
	for _, clause := range stmt.Claused {
		if err := clause.compile(w, tabs); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "}")
	return nil
}

type StatementSwitch struct {
	Location
	Keyword Location
	Init    xyz.Maybe[Statement]
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
	var init xyz.Maybe[Statement]
	if in.Init != nil {
		init = xyz.New(pkg.loadStatement(in.Init))
	}
	return StatementSwitch{
		Location: pkg.locations(in.Pos(), in.End()),
		Keyword:  pkg.location(in.Switch),
		Init:     init,
		Value:    value,
		Clauses:  clauses,
	}
}

func (stmt StatementSwitch) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "{")
	if init, ok := stmt.Init.Get(); ok {
		if err := init.compile(w, -tabs); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "switch (")
	if value, ok := stmt.Value.Get(); ok {
		if err := value.compile(w, tabs); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, ") {")
	for _, clause := range stmt.Clauses {
		if err := clause.compile(w, tabs+1); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
	fmt.Fprintf(w, "}}")
	return nil
}

type SwitchCaseClause struct {
	Location

	Keyword     Location
	Expressions []Expression
	Colon       Location
	Body        []Statement

	Fallsthrough bool
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
		stmt := pkg.loadStatement(stmt)
		if xyz.ValueOf(stmt) == Statements.Fallthrough {
			out.Fallsthrough = true
			break
		}
		out.Body = append(out.Body, stmt)
	}
	return out
}

func (clause SwitchCaseClause) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
	if len(clause.Expressions) == 0 {
		fmt.Fprintf(w, "else")
	} else {
		for i, expr := range clause.Expressions {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			if err := expr.compile(w, tabs); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(w, " => {")
	for _, stmt := range clause.Body {
		if err := stmt.compile(w, tabs+1); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
	fmt.Fprintf(w, "},")
	return nil
}

type StatementFallthrough struct {
	Location
}

func (pkg *Package) loadStatementFallthrough(in *ast.BranchStmt) StatementFallthrough {
	return StatementFallthrough{
		Location: pkg.locations(in.Pos(), in.End()),
	}
}

func (stmt StatementFallthrough) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "// fallthrough")
	return nil
}
