package zigc

import (
	"fmt"
	"go/types"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func (zig Target) StatementGo(stmt source.StatementGo) error {
	return zig.ExpressionCall(stmt.Call)
}

func (zig Target) ExpressionCall(expr source.ExpressionCall) error {
	function := expr.Function
	if xyz.ValueOf(function) == source.Expressions.Parenthesized {
		function = source.Expressions.Parenthesized.Get(function).X
	}
	var receiver xyz.Maybe[source.Expression]
	var variable bool
	var isInterface bool
	switch xyz.ValueOf(function) {
	case source.Expressions.BuiltinFunction:
		call := source.Expressions.BuiltinFunction.Get(function)
		switch call.String {
		case "println":
			return zig.println(expr)
		case "new":
			return zig.new(expr)
		case "make":
			return zig.make(expr)
		case "append":
			return zig.append(expr)
		case "copy":
			return zig.copy(expr)
		case "clear":
			return zig.clear(expr)
		case "len":
			return zig.len(expr)
		case "cap":
			return zig.cap(expr)
		case "panic":
			return zig.panic(expr)
		default:
			return expr.Errorf("unsupported builtin function %s", call)
		}
	case source.Expressions.DefinedFunction:
		call := source.Expressions.DefinedFunction.Get(function)
		if err := zig.DefinedFunction(call); err != nil {
			return err
		}
		if !call.Package {
			variable = true
		}
	case source.Expressions.DefinedVariable:
		call := source.Expressions.DefinedVariable.Get(function)
		if err := zig.DefinedVariable(call); err != nil {
			return err
		}
		if !call.Package {
			variable = true
		}
	case source.Expressions.Selector:
		left := source.Expressions.Selector.Get(function)
		if xyz.ValueOf(left.Selection) == source.Expressions.DefinedFunction {
			defined := source.Expressions.DefinedFunction.Get(left.Selection)
			if defined.Method {
				_, isInterface = left.X.TypeAndValue().Type.Underlying().(*types.Interface)
				if isInterface {
					if err := zig.Expression(left.X); err != nil {
						return err
					}
					fmt.Fprintf(zig, `.itype.`)
					if err := zig.DefinedFunction(defined); err != nil {
						return err
					}
					fmt.Fprintf(zig, `(goto, `)
					if err := zig.Expression(left.X); err != nil {
						return err
					}
					fmt.Fprintf(zig, ".value")
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
					fmt.Fprintf(zig, `%s.@"%s.`, zig.PackageOf(named.Obj().Pkg().Name()), named.Obj().Name())
					if err := zig.DefinedFunction(defined); err != nil {
						return err
					}
					fmt.Fprintf(zig, `"`)
				}
			} else {
				if err := zig.Compile(left); err != nil {
					return err
				}
			}
		} else {
			if err := zig.Compile(left); err != nil {
				return err
			}
		}
	case source.Expressions.Type:
		ctype := source.Expressions.Type.Get(function)
		switch typ := ctype.TypeAndValue().Type.Underlying().(type) {
		case *types.Interface:
			fmt.Fprintf(zig, "%s.make(goto,", zig.Type(ctype))
			if err := zig.Expression(expr.Arguments[0]); err != nil {
				return err
			}
			fmt.Fprintf(zig, ", %s, .{", zig.ReflectTypeOf(expr.Arguments[0].TypeAndValue().Type))
			for i := range typ.NumMethods() {
				if i > 0 {
					fmt.Fprintf(zig, ", ")
				}
				method := typ.Method(i)
				named := expr.Arguments[0].TypeAndValue().Type.(*types.Named)
				fmt.Fprintf(zig, `.%s = &@"%s.%[1]s.(itfc)"`, method.Name(), named.Obj().Pkg().Name()+"."+named.Obj().Name())
			}
			fmt.Fprintf(zig, "})")
			return nil
		default:
			fmt.Fprintf(zig, "@as(%s)", zig.Type(ctype))
			return nil
		}
	case source.Expressions.Function:
		if err := zig.Expression(function); err != nil {
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
			fmt.Fprintf(zig, ".go(.{null")
		} else if variable {
			fmt.Fprintf(zig, ".call(.{goto")
		} else {
			fmt.Fprintf(zig, "(goto")
		}
	}
	if receiver, ok := receiver.Get(); ok {
		fmt.Fprintf(zig, ", ")
		if err := zig.Expression(receiver); err != nil {
			return err
		}
	}
	var variadic bool
	for i, arg := range expr.Arguments {
		fmt.Fprintf(zig, ", ")
		if !variadic && (ftype.Variadic() && i >= ftype.Params().Len()-1) {
			fmt.Fprintf(zig, "go.variadic(%d, %s, .{", len(expr.Arguments)+1-ftype.Params().Len(), zig.TypeOf(ftype.Params().At(ftype.Params().Len()-1).Type().(*types.Slice).Elem()))
			variadic = true
		}
		if err := zig.Expression(arg); err != nil {
			return err
		}
	}
	if ftype.Variadic() {
		if variadic {
			fmt.Fprintf(zig, "})")
		} else {
			fmt.Fprintf(zig, ".{}")
		}
	}
	if variable {
		fmt.Fprintf(zig, "})")
	} else {
		fmt.Fprintf(zig, ")")
	}
	return nil
}
