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
	function := expr.Function
	if xyz.ValueOf(function) == Expressions.Parenthesized {
		function = Expressions.Parenthesized.Get(function).X
	}
	var variable bool
	switch xyz.ValueOf(function) {
	case Expressions.BuiltinFunction:
		call := Expressions.BuiltinFunction.Get(function)
		switch call.String() {
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
		case "len":
			return expr.len(w, tabs)
		case "cap":
			return expr.cap(w, tabs)
		default:
			return expr.Errorf("unsupported builtin function %s", call)
		}
	case Expressions.Identifier:
		call := Expressions.Identifier.Get(function)
		if err := call.compile(w, tabs); err != nil {
			return err
		}
		if !call.Package {
			variable = true
		}
	case Expressions.Selector:
		left := Expressions.Selector.Get(function)
		if err := left.compile(w, tabs); err != nil {
			return err
		}
	case Expressions.Type:
		fmt.Fprintf(w, "@as(")
		if err := Expressions.Type.Get(function).compile(w, tabs); err != nil {
			return err
		}
	default:
		return expr.Opening.Errorf("unsupported call for function of type %T", xyz.ValueOf(function))
	}
	ftype := expr.Function.TypeAndValue().Type.(*types.Signature)
	if variable && expr.Go {
		fmt.Fprintf(w, ".go(.{null")
	} else if variable {
		fmt.Fprintf(w, ".call(.{goto")
	} else {
		fmt.Fprintf(w, "(goto")
	}
	var variadic bool
	for i, arg := range expr.Arguments {
		fmt.Fprintf(w, ", ")
		if !variadic && (ftype.Variadic() && i >= ftype.Params().Len()-1) {
			fmt.Fprintf(w, "go.variadic(%d, %s, .{", len(expr.Arguments)+1-ftype.Params().Len(), zigTypeOf(ftype.Params().At(ftype.Params().Len()-1).Type().(*types.Slice).Elem()))
			variadic = true
		}
		if err := arg.compile(w, tabs); err != nil {
			return err
		}
	}

	if ftype.Variadic() {
		if variadic {
			fmt.Fprintf(w, "})")
		} else {
			fmt.Fprintf(w, ".{}")
		}
	}
	if variable {
		fmt.Fprintf(w, "})")
	} else {
		fmt.Fprintf(w, ")")
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
	case *types.Chan:
		switch len(expr.Arguments) {
		case 1, 2:
		default:
			return expr.Errorf("make expects one or two arguments, got %d", len(expr.Arguments))
		}
		fmt.Fprintf(w, "go.chan(%s).make(goto,", zigTypeOf(typ.Elem()))
		if len(expr.Arguments) == 2 {
			if err := expr.Arguments[1].compile(w, tabs); err != nil {
				return err
			}
		} else {
			fmt.Fprintf(w, "0")
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

func (expr ExpressionCall) len(w io.Writer, tabs int) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("len expects exactly one argument, got %d", len(expr.Arguments))
	}
	if err := expr.Arguments[0].compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, ".len()")
	return nil
}

func (expr ExpressionCall) cap(w io.Writer, tabs int) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("cap expects exactly one argument, got %d", len(expr.Arguments))
	}
	if err := expr.Arguments[0].compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, ".cap()")
	return nil
}
