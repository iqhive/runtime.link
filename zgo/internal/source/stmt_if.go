package source

import (
	"fmt"
	"go/ast"
	"io"
	"strings"

	"runtime.link/xyz"
)

type StatementIf struct {
	Location
	Keyword   Location
	Init      xyz.Maybe[Statement]
	Condition Expression
	Body      StatementBlock
	Else      xyz.Maybe[Statement]
}

func (pkg *Package) loadStatementIf(in *ast.IfStmt) StatementIf {
	var init xyz.Maybe[Statement]
	if in.Init != nil {
		init = xyz.New(pkg.loadStatement(in.Init))
	}
	return StatementIf{
		Location:  pkg.locations(in.Pos(), in.End()),
		Keyword:   pkg.location(in.If),
		Init:      init,
		Condition: pkg.loadExpression(in.Cond),
		Body:      pkg.loadStatementBlock(in.Body),
		Else:      xyz.New(pkg.loadStatement(in.Else)),
	}
}

func (stmt StatementIf) compile(w io.Writer, tabs int) error {
	init, hasInit := stmt.Init.Get()
	if hasInit {
		fmt.Fprintf(w, "{")
		initStmt, _ := init.Get()
		if err := initStmt.compile(w, tabs); err != nil {
			return err
		}
		fmt.Fprintf(w, "; ")
	}
	fmt.Fprintf(w, "if (")
	if err := stmt.Condition.compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, ") {")
	for _, stmt := range stmt.Body.Statements {
		if err := stmt.compile(w, tabs+1); err != nil {
			return err
		}
	}
	ifelse, hasElse := stmt.Else.Get()
	if hasElse {
		fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
		fmt.Fprintf(w, "} else ")
		if err := ifelse.compile(w, -tabs); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(w, "}")
	}
	return nil
}
