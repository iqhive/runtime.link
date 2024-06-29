package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"strings"

	"runtime.link/xyz"
)

type StatementFor struct {
	Location
	Keyword   Location
	Label     string
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
	if stmt.Label != "" {
		fmt.Fprintf(w, " %s:", stmt.Label)
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
	fmt.Fprintf(w, ")")
	statement, hasStatement := stmt.Statement.Get()
	if hasStatement {
		fmt.Fprintf(w, ": (")
		stmt, _ := statement.Get()
		if err := stmt.compile(w, -1); err != nil {
			return err
		}
		fmt.Fprintf(w, ")")
	}
	fmt.Fprintf(w, " {")
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

	Label string

	For     Location
	Key     xyz.Maybe[Expression]
	Value   xyz.Maybe[Expression]
	Token   WithLocation[token.Token]
	Keyword Location
	X       Expression
	Body    StatementBlock
}

func (pkg *Package) loadStatementRange(in *ast.RangeStmt) StatementRange {
	var key xyz.Maybe[Expression]
	if in.Key != nil {
		key = xyz.New(pkg.loadExpression(in.Key))
	}
	var val xyz.Maybe[Expression]
	if in.Value != nil {
		val = xyz.New(pkg.loadExpression(in.Value))
	}
	return StatementRange{
		Location: pkg.locations(in.Pos(), in.End()),
		For:      pkg.location(in.For),
		Key:      key,
		Value:    val,
		Token:    WithLocation[token.Token]{Value: in.Tok, SourceLocation: pkg.location(in.TokPos)},
		Keyword:  pkg.location(in.Range),
		X:        pkg.loadExpression(in.X),
		Body:     pkg.loadStatementBlock(in.Body),
	}
}

func (stmt StatementRange) compile(w io.Writer, tabs int) error {
	switch stmt.X.TypeAndValue().Type.(type) {
	case *types.Basic:
		fmt.Fprintf(w, "for (0..@as(%s,", zigTypeOf(stmt.X.TypeAndValue().Type))
		if err := stmt.X.compile(w, tabs); err != nil {
			return err
		}
		fmt.Fprintf(w, "))")
		key, hasKey := stmt.Key.Get()
		if hasKey {
			fmt.Fprintf(w, " |%s| ", key)
		} else {
			fmt.Fprintf(w, " |_|")
		}
		if stmt.Label != "" {
			fmt.Fprintf(w, " %s:", stmt.Label)
		}
		fmt.Fprintf(w, " {")
		for _, stmt := range stmt.Body.Statements {
			if err := stmt.compile(w, tabs+1); err != nil {
				return err
			}
		}
		fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
		fmt.Fprintf(w, "}")
		return nil
	case *types.Slice:
		fmt.Fprintf(w, "for (")
		key, hasKey := stmt.Key.Get()
		if key.String() == "_" {
			hasKey = false
		}
		val, hasVal := stmt.Value.Get()
		if hasKey {
			fmt.Fprintf(w, "0..,")
		}
		if err := stmt.X.compile(w, tabs); err != nil {
			return err
		}
		fmt.Fprintf(w, ".arraylist.items) |")
		if hasKey {
			fmt.Fprintf(w, "%s", key)
		}
		if hasKey && hasVal {
			fmt.Fprintf(w, ",")
		}
		if hasVal {
			fmt.Fprintf(w, "%s", val)
		}
		fmt.Fprintf(w, "|")
		if stmt.Label != "" {
			fmt.Fprintf(w, " %s:", stmt.Label)
		}
		fmt.Fprintf(w, " {")
		for _, stmt := range stmt.Body.Statements {
			if err := stmt.compile(w, tabs+1); err != nil {
				return err
			}
		}
		fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
		fmt.Fprintf(w, "}")
		return nil
	}
	return stmt.Errorf("range over unsupported type %T", stmt.X.TypeAndValue().Type)
}

type StatementContinue struct {
	Location

	Label xyz.Maybe[Identifier]
}

func (pkg *Package) loadStatementContinue(in *ast.BranchStmt) StatementContinue {
	var label xyz.Maybe[Identifier]
	if in.Label != nil {
		label = xyz.New(pkg.loadIdentifier(in.Label))
	}
	return StatementContinue{
		Location: pkg.locations(in.Pos(), in.End()),
		Label:    label,
	}
}

func (stmt StatementContinue) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "continue")
	label, hasLabel := stmt.Label.Get()
	if hasLabel {
		fmt.Fprintf(w, " :")
		if err := label.compile(w, tabs); err != nil {
			return err
		}
	}
	return nil
}

type StatementBreak struct {
	Location

	Label xyz.Maybe[Identifier]
}

func (pkg *Package) loadStatementBreak(in *ast.BranchStmt) StatementBreak {
	var label xyz.Maybe[Identifier]
	if in.Label != nil {
		label = xyz.New(pkg.loadIdentifier(in.Label))
	}
	return StatementBreak{
		Location: pkg.locations(in.Pos(), in.End()),
		Label:    label,
	}
}

func (stmt StatementBreak) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "break")
	label, hasLabel := stmt.Label.Get()
	if hasLabel {
		fmt.Fprintf(w, " :")
		if err := label.compile(w, tabs); err != nil {
			return err
		}
	}
	return nil
}
