package zigc

import (
	"fmt"
	"go/token"
	"go/types"
	"strings"

	"runtime.link/zgo/internal/source"
)

func (zig Target) Expression(expr source.Expression) error {
	e, _ := expr.Get()
	return zig.Compile(e)
}

func (zig Target) ImportedPackage(id source.ImportedPackage) error {
	fmt.Fprintf(zig, "%s", zig.PackageOf(id.String))
	return nil
}

func (zig Target) ExpressionBinary(expr source.ExpressionBinary) error {
	switch expr.Operation.Value {
	case token.NEQ:
		switch etype := expr.X.TypeAndValue().Type.(type) {
		case *types.Basic:
			switch etype.Kind() {
			case types.String, types.UntypedString:
				fmt.Fprintf(zig, "(!go.equality(%s, %s,%s))", zig.TypeOf(expr.X.TypeAndValue().Type), zig.toString(expr.X), zig.toString(expr.Y))
				return nil
			}
		}
	}
	if err := zig.Expression(expr.X); err != nil {
		return err
	}
	switch expr.Operation.Value {
	case token.LOR:
		fmt.Fprintf(zig, " or ")
	case token.LAND:
		fmt.Fprintf(zig, " and ")
	default:
		fmt.Fprintf(zig, " %s ", expr.Operation.Value)
	}
	return zig.Expression(expr.Y)
}

func (zig Target) Parenthesized(par source.Parenthesized) error {
	fmt.Fprintf(zig, "(")
	if err := zig.Expression(par.X); err != nil {
		return err
	}
	fmt.Fprintf(zig, ")")
	return nil
}

func (zig Target) ExpressionFunction(e source.ExpressionFunction) error {
	if zig.Tabs < 0 {
		zig.Tabs = -zig.Tabs
	}
	fmt.Fprintf(zig, "%s.make(&struct{pub fn call(package: *const anyopaque, default: ?*go.routine", zig.TypeOf(e.Type.TypeAndValue().Type))
	for _, arg := range e.Type.Arguments.Fields {
		names, ok := arg.Names.Get()
		if ok {
			for _, name := range names {
				fmt.Fprintf(zig, ",%s: %s", zig.toString(name), zig.Type(arg.Type))
			}
		} else {
			fmt.Fprintf(zig, ",_: %s", zig.Type(arg.Type))
		}
	}
	fmt.Fprintf(zig, ") ")
	results, ok := e.Type.Results.Get()
	if !ok {
		fmt.Fprintf(zig, "void")
	} else {
		switch len(results.Fields) {
		case 1:
			fmt.Fprintf(zig, "%s", zig.Type(results.Fields[0].Type))
		default:
			return e.Errorf("multiple return values not supported")
		}
	}
	fmt.Fprintf(zig, " { var chan2 = go.routine{}; const goto2: *go.routine = if (default) |select| select else &chan2; if (default == null) {defer goto2.exit();} go.use(package);")
	for _, stmt := range e.Body.Statements {
		zig.Tabs++
		if err := zig.Statement(stmt); err != nil {
			return err
		}
		zig.Tabs--
	}
	fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
	fmt.Fprintf(zig, "}}{})")
	return nil
}

func (zig Target) ExpressionIndex(expr source.ExpressionIndex) error {
	switch expr.X.TypeAndValue().Type.(type) {
	case *types.Slice:
		if err := zig.Expression(expr.X); err != nil {
			return err
		}
		fmt.Fprintf(zig, ".index(")
		if err := zig.Expression(expr.Index); err != nil {
			return err
		}
		fmt.Fprintf(zig, ")")
		return nil
	case *types.Map:
		if err := zig.Expression(expr.X); err != nil {
			return err
		}
		fmt.Fprintf(zig, ".get(")
		if err := zig.Expression(expr.Index); err != nil {
			return err
		}
		fmt.Fprintf(zig, ")")
		return nil
	case *types.Array:
		if err := zig.Expression(expr.X); err != nil {
			return err
		}
		fmt.Fprintf(zig, "[")
		if err := zig.Expression(expr.Index); err != nil {
			return err
		}
		fmt.Fprintf(zig, "]")
		return nil
	default:
		return fmt.Errorf("unsupported index of type %T", expr)
	}
}

func (zig Target) ExpressionKeyValue(e source.ExpressionKeyValue) error {
	fmt.Fprintf(zig, ".")
	if err := zig.Expression(e.Key); err != nil {
		return err
	}
	fmt.Fprintf(zig, "=")
	if err := zig.Expression(e.Value); err != nil {
		return err
	}
	return nil
}

func (zig Target) AwaitChannel(e source.AwaitChannel) error {
	if err := zig.Expression(e.Chan); err != nil {
		return err
	}
	fmt.Fprint(zig, ".recv(goto)")
	return nil
}

func (zig Target) ExpressionSlice(e source.ExpressionSlice) error {
	if err := zig.Expression(e.X); err != nil {
		return err
	}
	fmt.Fprintf(zig, ".range(")
	from, ok := e.From.Get()
	if ok {
		if err := zig.Expression(from); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(zig, "0")
	}
	fmt.Fprintf(zig, ", ")
	high, ok := e.High.Get()
	if ok {
		if err := zig.Expression(high); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(zig, "0")
	}
	fmt.Fprintf(zig, ")")
	return nil
}

func (zig Target) ExpressionTypeAssertion(e source.ExpressionTypeAssertion) error {
	if err := zig.Expression(e.X); err != nil {
		return err
	}
	fmt.Fprintf(zig, " .(%s)", e.Type)
	return nil
}

func (zig Target) ExpressionUnary(e source.ExpressionUnary) error {
	fmt.Fprintf(zig, "%s", e.Operation.Value)
	if err := zig.Expression(e.X); err != nil {
		return err
	}
	return nil
}
