package source

import (
	"fmt"
	"go/ast"
	"io"
	"strings"
)

type ExpressionFunction struct {
	typed

	Location

	Type TypeFunction
	Body StatementBlock
}

func (pkg *Package) loadExpressionFunction(in *ast.FuncLit) ExpressionFunction {
	var out ExpressionFunction
	out.Location = pkg.locations(in.Pos(), in.End())
	out.typed = pkg.typed(in)
	out.Type = pkg.loadTypeFunction(in.Type)
	out.Body = pkg.loadStatementBlock(in.Body)
	return out
}

func (e ExpressionFunction) compile(w io.Writer, tabs int) error {
	if tabs < 0 {
		tabs = -tabs
	}
	fmt.Fprintf(w, "%s.make(&struct{pub fn call(package: *const anyopaque, default: ?*go.routine", e.Type.ZigType())
	for _, arg := range e.Type.Arguments.Fields {
		names, ok := arg.Names.Get()
		if ok {
			for _, name := range names {
				fmt.Fprintf(w, ",%s: %s", toString(name), arg.Type.ZigType())
			}
		} else {
			fmt.Fprintf(w, ",_: %s", arg.Type.ZigType())
		}
	}
	fmt.Fprintf(w, ") ")
	results, ok := e.Type.Results.Get()
	if !ok {
		fmt.Fprintf(w, "void")
	} else {
		switch len(results.Fields) {
		case 1:
			fmt.Fprintf(w, "%s", results.Fields[0].Type.ZigType())
		default:
			return e.Errorf("multiple return values not supported")
		}
	}
	fmt.Fprintf(w, " { var chan2 = go.routine{}; const goto2: *go.routine = if (default) |select| select else &chan2; if (default == null) {defer goto2.exit();} go.use(package);")
	for _, stmt := range e.Body.Statements {
		if err := stmt.compile(w, tabs+1); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
	fmt.Fprintf(w, "}}{})")
	return nil
}
