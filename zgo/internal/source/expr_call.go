package source

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"

	"runtime.link/xyz"
)

type ExpressionCall struct {
	Location

	typed

	Go bool

	Function  Expression
	Opening   Location
	Arguments []Expression
	Ellipsis  Location
	Closing   Location
}

func (pkg *Package) loadExpressionCall(in *ast.CallExpr) ExpressionCall {
	var out ExpressionCall
	out.Location = pkg.locations(in.Pos(), in.End())
	out.typed = typed{pkg.Types[in]}
	out.Function = pkg.loadExpression(in.Fun)
	out.Opening = pkg.location(in.Lparen)
	for _, arg := range in.Args {
		out.Arguments = append(out.Arguments, pkg.loadExpression(arg))
	}
	out.Ellipsis = pkg.location(in.Ellipsis)
	out.Closing = pkg.location(in.Rparen)
	return out
}

func (expr ExpressionCall) compile(w io.Writer, tabs int) error {
	switch xyz.ValueOf(expr.Function) {
	case Expressions.BuiltinFunction:
		call := Expressions.BuiltinFunction.Get(expr.Function)
		switch call.Name.Value {
		case "println":
			return expr.println(w, tabs)
		case "new":
			return expr.new(w, tabs)
		case "make":
			return expr.make(w, tabs)
		case "append":
			return expr.append(w, tabs)
		case "copy":
			return expr.copy(w, tabs)
		case "clear":
			return expr.clear(w, tabs)
		default:
			return call.Name.SourceLocation.Errorf("unsupported builtin function %s", call.Name.Value)
		}
	case Expressions.Identifier:
		call := Expressions.Identifier.Get(expr.Function)
		if expr.Go {
			fmt.Fprintf(w, "%s.go(.{null", call.Name.Value)
		} else {
			fmt.Fprintf(w, "%s(goto", call.Name.Value)
		}
		for _, arg := range expr.Arguments {
			fmt.Fprintf(w, ", ")
			if err := arg.compile(w, tabs); err != nil {
				return err
			}
		}
		if expr.Go {
			fmt.Fprintf(w, "})")
		} else {
			fmt.Fprintf(w, ")")
		}
	default:
		return expr.Opening.Errorf("unsupported call for function of type %T", expr)
	}
	return nil
}

func (expr ExpressionCall) println(w io.Writer, tabs int) error {
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
				return expr.Location.Errorf("unsupported type %s", rtype)
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
		if err := arg.compile(w, tabs); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "})")
	return nil
}

func (expr ExpressionCall) new(w io.Writer, tabs int) error {
	if len(expr.Arguments) != 1 {
		return expr.Errorf("new expects exactly one argument, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(w, "go.new(goto, %s)", zigTypeOf(expr.Arguments[0].TypeAndValue().Type))
	return nil
}

func (expr ExpressionCall) make(w io.Writer, tabs int) error {
	switch typ := expr.Arguments[0].TypeAndValue().Type.(type) {
	case *types.Slice:
		switch len(expr.Arguments) {
		case 2, 3:
		default:
			return expr.Errorf("make expects two or three arguments, got %d", len(expr.Arguments))
		}
		fmt.Fprintf(w, "go.slice(%s).make(goto,",
			zigTypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
		if err := expr.Arguments[1].compile(w, tabs); err != nil {
			return err
		}
		fmt.Fprintf(w, ",")
		if len(expr.Arguments) == 3 {
			if err := expr.Arguments[2].compile(w, tabs); err != nil {
				return err
			}
		} else {
			if err := expr.Arguments[1].compile(w, tabs); err != nil {
				return err
			}
		}
		fmt.Fprintf(w, ")")
		return nil
	case *types.Map:
		if len(expr.Arguments) != 1 {
			return expr.Errorf("make expects exactly one argument, got %d", len(expr.Arguments))
		}
		if typ.Key().String() == "string" {
			fmt.Fprintf(w, "go.smap(%s).make(goto, 0)", zigTypeOf(typ.Elem()))
		} else {
			fmt.Fprintf(w, "go.map(%s, %s).make(goto, 0)", zigTypeOf(typ.Key()), zigTypeOf(typ.Elem()))
		}
		return nil
	default:
		return fmt.Errorf("unsupported type %T", expr.Arguments[0].TypeAndValue().Type)
	}
}

func (expr ExpressionCall) append(w io.Writer, tabs int) error {
	if len(expr.Arguments) != 2 {
		return expr.Errorf("append expects exactly two arguments, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(w, "go.append(goto, %s, ", zigTypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
	if err := expr.Arguments[0].compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, ", ")
	if err := expr.Arguments[1].compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, ")")
	return nil
}

func (expr ExpressionCall) copy(w io.Writer, tabs int) error {
	if len(expr.Arguments) != 2 {
		return fmt.Errorf("copy expects exactly two arguments, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(w, "go.copy(%s,", zigTypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
	if err := expr.Arguments[0].compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, ", ")
	if err := expr.Arguments[1].compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, ")")
	return nil
}

func (expr ExpressionCall) clear(w io.Writer, tabs int) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("clear expects exactly one argument, got %d", len(expr.Arguments))
	}
	if err := expr.Arguments[0].compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, ".clear()")
	return nil
}
