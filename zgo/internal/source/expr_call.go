package source

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"

	"runtime.link/xyz"
)

type ExpressionCall struct {
	typed

	Function  Expression
	Opening   Location
	Arguments []Expression
	Ellipsis  Location
	Closing   Location
}

func (pkg *Package) loadExpressionCall(in *ast.CallExpr) ExpressionCall {
	var out ExpressionCall
	out.typed = typed{pkg.Types[in]}
	out.Function = pkg.loadExpression(in.Fun)
	out.Opening = Location(in.Lparen)
	for _, arg := range in.Args {
		out.Arguments = append(out.Arguments, pkg.loadExpression(arg))
	}
	out.Ellipsis = Location(in.Ellipsis)
	out.Closing = Location(in.Rparen)
	return out
}

func (expr ExpressionCall) compile(w io.Writer) error {
	switch xyz.ValueOf(expr.Function) {
	case Expressions.BuiltinFunction:
		call := Expressions.BuiltinFunction.Get(expr.Function)
		switch call.Name.Value {
		case "println":
			return expr.println(w)
		case "new":
			return expr.new(w)
		case "make":
			return expr.make(w)
		case "append":
			return expr.append(w)
		case "copy":
			return expr.copy(w)
		case "clear":
			return expr.clear(w)
		default:
			return fmt.Errorf("unsupported builtin function %s", call.Name.Value)
		}
	case Expressions.Identifier:
		call := Expressions.Identifier.Get(expr.Function)
		fmt.Fprintf(w, "%s(go", call.Name.Value)
		for _, arg := range expr.Arguments {
			fmt.Fprintf(w, ", ")
			if err := arg.compile(w); err != nil {
				return err
			}
		}
		fmt.Fprintf(w, ")")
	default:
		return fmt.Errorf("unsupported function of type %T", expr)
	}
	return nil
}

func (expr ExpressionCall) println(w io.Writer) error {
	fmt.Fprintf(w, "go.println(")
	var format string
	for i, arg := range expr.Arguments {
		if i > 0 {
			format += " "
		}
		switch rtype := arg.TypeAndValue().Type.(type) {
		case *types.Basic:
			switch rtype.Kind() {
			case types.Int, types.Int8, types.Int16, types.Int32, types.Int64, types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Uintptr:
				format += "{d}"
			case types.Float64, types.Float32:
				format += "{e}"
			case types.String:
				format += "{s}"
			default:
				return fmt.Errorf("unsupported type %s", rtype)
			}
		default:
			return fmt.Errorf("unsupported type %T", rtype)
		}
	}
	fmt.Fprintf(w, "\"%s\", .{", format)
	for i, arg := range expr.Arguments {
		if i > 0 {
			fmt.Fprintf(w, ", ")
		}
		if err := arg.compile(w); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "})")
	return nil
}

func (expr ExpressionCall) new(w io.Writer) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("new expects exactly one argument, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(w, "go.new(%s)", zigTypeOf(expr.Arguments[0].TypeAndValue().Type))
	return nil
}

func (expr ExpressionCall) make(w io.Writer) error {
	switch typ := expr.Arguments[0].TypeAndValue().Type.(type) {
	case *types.Slice:
		if len(expr.Arguments) != 2 {
			return fmt.Errorf("make expects exactly two arguments, got %d", len(expr.Arguments))
		}
		fmt.Fprintf(w, "go.makeSlice(%s, ", zigTypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
		if err := expr.Arguments[1].compile(w); err != nil {
			return err
		}
		fmt.Fprintf(w, ",")
		if err := expr.Arguments[1].compile(w); err != nil {
			return err
		}
		fmt.Fprintf(w, ")")
		return nil
	case *types.Map:
		if len(expr.Arguments) != 1 {
			return fmt.Errorf("make expects exactly one argument, got %d", len(expr.Arguments))
		}
		if typ.Key().String() == "string" {
			fmt.Fprintf(w, "go.make_smap(%s, 0)", zigTypeOf(typ.Elem()))
		} else {
			fmt.Fprintf(w, "go.make_map(%s, %s, 0)", zigTypeOf(typ.Key()), zigTypeOf(typ.Elem()))
		}
		return nil
	default:
		return fmt.Errorf("unsupported type %T", expr.Arguments[0].TypeAndValue().Type)
	}
}

func (expr ExpressionCall) append(w io.Writer) error {
	if len(expr.Arguments) != 2 {
		return fmt.Errorf("append expects exactly two arguments, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(w, "go.append(%s, ", zigTypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
	if err := expr.Arguments[0].compile(w); err != nil {
		return err
	}
	fmt.Fprintf(w, ", ")
	if err := expr.Arguments[1].compile(w); err != nil {
		return err
	}
	fmt.Fprintf(w, ")")
	return nil
}

func (expr ExpressionCall) copy(w io.Writer) error {
	if len(expr.Arguments) != 2 {
		return fmt.Errorf("copy expects exactly two arguments, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(w, "go.copy(%s,", zigTypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
	if err := expr.Arguments[0].compile(w); err != nil {
		return err
	}
	fmt.Fprintf(w, ", ")
	if err := expr.Arguments[1].compile(w); err != nil {
		return err
	}
	fmt.Fprintf(w, ")")
	return nil
}

func (expr ExpressionCall) clear(w io.Writer) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("clear expects exactly one argument, got %d", len(expr.Arguments))
	}

	switch rtype := expr.Arguments[0].TypeAndValue().Type.(type) {
	case *types.Slice:
		fmt.Fprintf(w, "@memset(")
		if err := expr.Arguments[0].compile(w); err != nil {
			return err
		}
		fmt.Fprintf(w, ".items, std.mem.zeroes(%s))", zigTypeOf(rtype.Elem()))
	case *types.Map:
		if err := expr.Arguments[0].compile(w); err != nil {
			return err
		}
		fmt.Fprintf(w, ".clear()")
	default:
		return fmt.Errorf("unsupported type %T", expr.Arguments[0].TypeAndValue().Type)
	}
	return nil
}
