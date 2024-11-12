package zigc

import (
	"fmt"
	"go/types"
	"strings"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func (zig Target) Statement(stmt source.Statement) error {
	switch xyz.ValueOf(stmt) {
	case source.Statements.Declaration:
	default:
		if zig.Tabs >= 0 {
			fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
		}
	}
	if zig.Tabs < 0 {
		zig.Tabs = -zig.Tabs
	}
	if xyz.ValueOf(stmt) == source.Statements.Expression {
		expr := source.Statements.Expression.Get(stmt)
		switch expr := expr.TypeAndValue().Type.(type) {
		case *types.Basic:
			fmt.Fprintf(zig, "_ = ")
		case *types.Tuple:
			if expr.Len() == 0 {
				break
			}
			for i := 0; i < expr.Len(); i++ {
				if i > 0 {
					fmt.Fprintf(zig, ", ")
				}
				fmt.Fprintf(zig, "_")
			}
			fmt.Fprintf(zig, " = ")
		default:
			return fmt.Errorf("unsupported expression type %T", expr)
		}
	}
	value, _ := stmt.Get()
	if err := zig.Compile(value); err != nil {
		return err
	}
	switch xyz.ValueOf(stmt) {
	case source.Statements.Block, source.Statements.Empty, source.Statements.For, source.Statements.Range,
		source.Statements.If, source.Statements.Declaration, source.Statements.Switch:
		return nil
	default:
		fmt.Fprintf(zig, ";")
		return nil
	}
}

func (zig Target) StatementBlock(stmt source.StatementBlock) error {
	fmt.Fprintf(zig, "{")
	for _, stmt := range stmt.Statements {
		zig.Tabs++
		if err := zig.Statement(stmt); err != nil {
			return err
		}
		zig.Tabs--
	}
	fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
	fmt.Fprintf(zig, "}")
	return nil
}

func (zig Target) StatementDecrement(stmt source.StatementDecrement) error {
	value, _ := stmt.WithLocation.Value.Get()
	if err := zig.Compile(value); err != nil {
		return err
	}
	fmt.Fprintf(zig, "-=1")
	return nil
}

func (zig Target) StatementDefer(stmt source.StatementDefer) error {
	// TODO arguments need to be evaluated at the time of the defer statement.
	if stmt.OutermostScope {
		fmt.Fprintf(zig, "defer ")
		return zig.ExpressionCall(stmt.Call)
	}
	return stmt.Location.Errorf("only defer at the outermost scope of a function is currently supported")
}

func (zig Target) StatementEmpty(stmt source.StatementEmpty) error { return nil }

func (zig Target) StatementBreak(stmt source.StatementBreak) error {
	fmt.Fprintf(zig, "break")
	label, hasLabel := stmt.Label.Get()
	if hasLabel {
		fmt.Fprintf(zig, " :")
		if err := zig.Identifier(label); err != nil {
			return err
		}
	}
	return nil
}

func (zig Target) StatementIncrement(stmt source.StatementIncrement) error {
	if err := zig.Expression(stmt.WithLocation.Value); err != nil {
		return err
	}
	fmt.Fprintf(zig, "+=1")
	return nil
}

func (zig Target) StatementReturn(stmt source.StatementReturn) error {
	fmt.Fprintf(zig, "return")
	for _, result := range stmt.Results {
		fmt.Fprintf(zig, " ")
		if err := zig.Expression(result); err != nil {
			return err
		}
	}
	return nil
}

func (zig Target) StatementSend(stmt source.StatementSend) error {
	if err := zig.Expression(stmt.X); err != nil {
		return err
	}
	fmt.Fprint(zig, ".send(goto,")
	if err := zig.Expression(stmt.Value); err != nil {
		return err
	}
	fmt.Fprint(zig, ")")
	return nil
}

func (zig Target) StatementSwitchType(stmt source.StatementSwitchType) error {
	fmt.Fprintf(zig, "switch ")
	if init, ok := stmt.Init.Get(); ok {
		if err := zig.Statement(init); err != nil {
			return err
		}
	}
	if err := zig.Statement(stmt.Assign); err != nil {
		return err
	}
	fmt.Fprintf(zig, " {")
	for _, clause := range stmt.Claused {
		if err := zig.SwitchCaseClause(clause); err != nil {
			return err
		}
	}
	fmt.Fprintf(zig, "}")
	return nil
}

func (zig Target) SwitchCaseClause(clause source.SwitchCaseClause) error {
	fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
	if len(clause.Expressions) == 0 {
		fmt.Fprintf(zig, "else")
	} else {
		for i, expr := range clause.Expressions {
			if i > 0 {
				fmt.Fprintf(zig, ", ")
			}
			if err := zig.Expression(expr); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(zig, " => {")
	for _, stmt := range clause.Body {
		zig.Tabs++
		if err := zig.Statement(stmt); err != nil {
			return err
		}
		zig.Tabs--
	}
	fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
	fmt.Fprintf(zig, "},")
	return nil
}
