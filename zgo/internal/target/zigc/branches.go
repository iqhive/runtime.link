package zigc

import (
	"fmt"
	"strings"

	"runtime.link/zgo/internal/source"
)

func (zig Target) StatementIf(stmt source.StatementIf) error {
	init, hasInit := stmt.Init.Get()
	if hasInit {
		fmt.Fprintf(zig, "{")
		if err := zig.Statement(init); err != nil {
			return err
		}
		fmt.Fprintf(zig, "; ")
	}
	fmt.Fprintf(zig, "if (")
	if err := zig.Expression(stmt.Condition); err != nil {
		return err
	}
	fmt.Fprintf(zig, ") {")
	for _, stmt := range stmt.Body.Statements {
		zig.Tabs++
		if err := zig.Statement(stmt); err != nil {
			return err
		}
		zig.Tabs--
	}
	ifelse, hasElse := stmt.Else.Get()
	if hasElse {
		fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
		fmt.Fprintf(zig, "} else ")
		zig.Tabs = -zig.Tabs
		if err := zig.Statement(ifelse); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
		fmt.Fprintf(zig, "}")
	}
	return nil
}

func (zig Target) StatementSwitch(stmt source.StatementSwitch) error {
	fmt.Fprintf(zig, "{")
	if init, ok := stmt.Init.Get(); ok {
		zig.Tabs = -zig.Tabs
		if err := zig.Statement(init); err != nil {
			return err
		}
		zig.Tabs = -zig.Tabs
	}
	fmt.Fprintf(zig, "switch (")
	if value, ok := stmt.Value.Get(); ok {
		if err := zig.Expression(value); err != nil {
			return err
		}
	}
	fmt.Fprintf(zig, ") {")
	for _, clause := range stmt.Clauses {
		zig.Tabs++
		if err := zig.SwitchCaseClause(clause); err != nil {
			return err
		}
		zig.Tabs--
	}
	fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
	fmt.Fprintf(zig, "}}")
	return nil
}
