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
	out.typed = pkg.typed(in)
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
	var receiver xyz.Maybe[Expression]
	var variable bool
	var isInterface bool
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
		case "panic":
			return expr.panic(w, tabs)
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
		if left.Selection.Method {
			_, isInterface = left.X.TypeAndValue().Type.Underlying().(*types.Interface)
			if isInterface {
				if err := left.X.compile(w, tabs); err != nil {
					return err
				}
				fmt.Fprintf(w, `.itype.%s(goto, `, left.Selection.String())
				if err := left.X.compile(w, tabs); err != nil {
					return err
				}
				fmt.Fprintf(w, ".value")
			} else {
				receiver = xyz.New(left.X)
				rtype := left.X.TypeAndValue().Type
				for {
					pointer, ok := rtype.Underlying().(*types.Pointer)
					if !ok {
						break
					}
					rtype = pointer.Elem()
				}
				named, ok := rtype.(*types.Named)
				if !ok {
					return left.Errorf("unsupported receiver type %s", rtype)
				}
				fmt.Fprintf(w, `%s.@"%s.%s"`, zigPackageOf(named.Obj().Pkg().Name()), named.Obj().Name(), left.Selection.String())
			}
		} else {
			if err := left.compile(w, tabs); err != nil {
				return err
			}
		}
	case Expressions.Type:
		ctype := Expressions.Type.Get(function)
		switch typ := ctype.TypeAndValue().Type.Underlying().(type) {
		case *types.Interface:
			fmt.Fprintf(w, "%s.make(goto,", ctype.ZigType())
			if err := expr.Arguments[0].compile(w, tabs); err != nil {
				return err
			}
			fmt.Fprintf(w, ", %s, .{", expr.Arguments[0].ZigReflectType())
			for i := range typ.NumMethods() {
				if i > 0 {
					fmt.Fprintf(w, ", ")
				}
				method := typ.Method(i)
				named := expr.Arguments[0].TypeAndValue().Type.(*types.Named)
				fmt.Fprintf(w, `.%s = &@"%s.%[1]s.(itfc)"`, method.Name(), named.Obj().Pkg().Name()+"."+named.Obj().Name())
			}
			fmt.Fprintf(w, "})")
			return nil
		default:
			fmt.Fprintf(w, "@as(")
			if err := ctype.compile(w, tabs); err != nil {
				return err
			}
			fmt.Fprintf(w, ")")
			return nil
		}
	case Expressions.Function:
		if err := function.compile(w, tabs); err != nil {
			return err
		}
	default:
		return expr.Opening.Errorf("unsupported call for function of type %T", xyz.ValueOf(function))
	}
	ftype, ok := expr.Function.TypeAndValue().Type.(*types.Signature)
	if !ok {
		return expr.Errorf("unsupported function type %T", expr.Function.TypeAndValue().Type)
	}
	if !isInterface {
		if variable && expr.Go {
			fmt.Fprintf(w, ".go(.{null")
		} else if variable {
			fmt.Fprintf(w, ".call(.{goto")
		} else {
			fmt.Fprintf(w, "(goto")
		}
	}
	if receiver, ok := receiver.Get(); ok {
		fmt.Fprintf(w, ", ")
		if err := receiver.compile(w, tabs); err != nil {
			return err
		}
	}
	var variadic bool
	for i, arg := range expr.Arguments {
		fmt.Fprintf(w, ", ")
		if !variadic && (ftype.Variadic() && i >= ftype.Params().Len()-1) {
			fmt.Fprintf(w, "go.variadic(%d, %s, .{", len(expr.Arguments)+1-ftype.Params().Len(), expr.typed.zigTypeOf(ftype.Params().At(ftype.Params().Len()-1).Type().(*types.Slice).Elem()))
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
			case types.Bool:
				format += "{}"
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
	fmt.Fprintf(w, "go.new(goto, %s)", expr.Arguments[0].ZigType())
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
			expr.typed.zigTypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
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
		fmt.Fprintf(w, "go.chan(%s).make(goto,", expr.typed.zigTypeOf(typ.Elem()))
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
			fmt.Fprintf(w, "go.smap(%s).make(goto, 0)", expr.typed.zigTypeOf(typ.Elem()))
		} else {
			fmt.Fprintf(w, "go.map(%s, %s).make(goto, 0)", expr.typed.zigTypeOf(typ.Key()), expr.typed.zigTypeOf(typ.Elem()))
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
	fmt.Fprintf(w, "go.append(goto, %s, ", expr.typed.zigTypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
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
	fmt.Fprintf(w, "go.copy(%s,", expr.typed.zigTypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
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

func (expr ExpressionCall) panic(w io.Writer, tabs int) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("panic expects exactly one argument, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(w, "@panic(")
	if err := expr.Arguments[0].compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, ")")
	return nil
}
