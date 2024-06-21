package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"strings"

	"runtime.link/xyz"
)

type StatementFor struct {
	Location
	Keyword   Location
	Init      xyz.Maybe[Statement]
	Condition xyz.Maybe[Expression]
	Statement xyz.Maybe[Statement]
	Body      StatementBlock
}

func (pkg *Package) loadStatementFor(in *ast.ForStmt) StatementFor {
	var init xyz.Maybe[Statement]
	if in.Init != nil {
		init = xyz.New(pkg.loadStatement(in.Init))
	}
	var cond xyz.Maybe[Expression]
	if in.Cond != nil {
		cond = xyz.New(pkg.loadExpression(in.Cond))
	}
	var stmt xyz.Maybe[Statement]
	if in.Post != nil {
		stmt = xyz.New(pkg.loadStatement(in.Post))
	}
	return StatementFor{
		Location:  pkg.locations(in.Pos(), in.End()),
		Keyword:   pkg.location(in.For),
		Init:      init,
		Condition: cond,
		Statement: stmt,
		Body:      pkg.loadStatementBlock(in.Body),
	}
}

func (stmt StatementFor) compile(w io.Writer, tabs int) error {
	init, hasInit := stmt.Init.Get()
	if hasInit {
		fmt.Fprintf(w, "{")
		if err := init.compile(w, -tabs); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "while (")
	condition, hasCondition := stmt.Condition.Get()
	if hasCondition {
		if err := condition.compile(w, tabs); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(w, "true")
	}
	fmt.Fprintf(w, ") {")
	statement, hasStatement := stmt.Statement.Get()
	if hasStatement {
		fmt.Fprintf(w, "defer ")
		if err := statement.compile(w, -1); err != nil {
			return err
		}
	}
	for _, stmt := range stmt.Body.Statements {
		if err := stmt.compile(w, tabs+1); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
	fmt.Fprintf(w, "}")
	if hasInit {
		fmt.Fprintf(w, "}")
	}
	return nil
}

type StatementRange struct {
	Location
	For        Location
	Key, Value Expression
	Token      WithLocation[token.Token]
	Keyword    Location
	X          Expression
	Body       StatementBlock
}

func (pkg *Package) loadStatementRange(in *ast.RangeStmt) StatementRange {
	return StatementRange{
		Location: pkg.locations(in.Pos(), in.End()),
		For:      pkg.location(in.For),
		Key:      pkg.loadExpression(in.Key),
		Value:    pkg.loadExpression(in.Value),
		Token:    WithLocation[token.Token]{Value: in.Tok, SourceLocation: pkg.location(in.TokPos)},
		Keyword:  pkg.location(in.Range),
		X:        pkg.loadExpression(in.X),
		Body:     pkg.loadStatementBlock(in.Body),
	}
}

func (stmt StatementRange) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "for (")
	if err := stmt.Value.compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, " := range ")
	if err := stmt.X.compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, ") {")
	for _, stmt := range stmt.Body.Statements {
		if err := stmt.compile(w, tabs+1); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
	fmt.Fprintf(w, "}")
	return nil
}
