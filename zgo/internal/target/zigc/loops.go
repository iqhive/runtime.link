package zigc

import (
	"fmt"
	"go/types"
	"strings"

	"runtime.link/zgo/internal/source"
)

func (zig Target) StatementFor(stmt source.StatementFor) error {
	init, hasInit := stmt.Init.Get()
	if hasInit {
		fmt.Fprintf(zig, "{")
		if err := zig.Statement(init); err != nil {
			return err
		}
	}
	if stmt.Label != "" {
		fmt.Fprintf(zig, " %s:", stmt.Label)
	}
	fmt.Fprintf(zig, "while (")
	condition, hasCondition := stmt.Condition.Get()
	if hasCondition {
		if err := zig.Expression(condition); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(zig, "true")
	}
	fmt.Fprintf(zig, ")")
	statement, hasStatement := stmt.Statement.Get()
	if hasStatement {
		fmt.Fprintf(zig, ": (")
		zig.Tabs = -zig.Tabs
		stmt, _ := statement.Get()
		if err := zig.Compile(stmt); err != nil {
			return err
		}
		zig.Tabs = -zig.Tabs
		fmt.Fprintf(zig, ")")
	}
	fmt.Fprintf(zig, " {")
	for _, stmt := range stmt.Body.Statements {
		zig.Tabs++
		if err := zig.Statement(stmt); err != nil {
			return err
		}
		zig.Tabs--
	}
	fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
	fmt.Fprintf(zig, "}")
	if hasInit {
		fmt.Fprintf(zig, "}")
	}
	return nil
}

func (zig Target) StatementRange(stmt source.StatementRange) error {
	switch stmt.X.TypeAndValue().Type.(type) {
	case *types.Basic:
		fmt.Fprintf(zig, "for (0..@as(%s,", zig.TypeOf(stmt.X.TypeAndValue().Type))
		if err := zig.Expression(stmt.X); err != nil {
			return err
		}
		fmt.Fprintf(zig, "))")
		key, hasKey := stmt.Key.Get()
		if hasKey {
			fmt.Fprintf(zig, " | ")
			if err := zig.DefinedVariable(key); err != nil {
				return err
			}
			fmt.Fprintf(zig, " |")
		} else {
			fmt.Fprintf(zig, " |_|")
		}
		if stmt.Label != "" {
			fmt.Fprintf(zig, " %s:", stmt.Label)
		}
		fmt.Fprintf(zig, " {")
		for _, stmt := range stmt.Body.Statements {
			zig.Tabs++
			if err := zig.Statement(stmt); err != nil {
				return err
			}
			zig.Tabs--
		}
		fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
		fmt.Fprintf(zig, "}")
		return nil
	case *types.Slice:
		fmt.Fprintf(zig, "for (")
		key, hasKey := stmt.Key.Get()
		if key.String == "_" {
			hasKey = false
		}
		val, hasVal := stmt.Value.Get()
		if hasKey {
			fmt.Fprintf(zig, "0..,")
		}
		if err := zig.Expression(stmt.X); err != nil {
			return err
		}
		fmt.Fprintf(zig, ".arraylist.items) |")
		if hasKey {
			if err := zig.DefinedVariable(key); err != nil {
				return err
			}
		}
		if hasKey && hasVal {
			fmt.Fprintf(zig, ",")
		}
		if hasVal {
			if err := zig.DefinedVariable(val); err != nil {
				return err
			}
		}
		fmt.Fprintf(zig, "|")
		if stmt.Label != "" {
			fmt.Fprintf(zig, " %s:", stmt.Label)
		}
		fmt.Fprintf(zig, " {")
		for _, stmt := range stmt.Body.Statements {
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
	return stmt.Errorf("range over unsupported type %T", stmt.X.TypeAndValue().Type)
}

func (zig Target) StatementContinue(stmt source.StatementContinue) error {
	fmt.Fprintf(zig, "continue")
	label, hasLabel := stmt.Label.Get()
	if hasLabel {
		fmt.Fprintf(zig, " : %s", label.String)
	}
	return nil
}
