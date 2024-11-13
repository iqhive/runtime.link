package zigc

import (
	"fmt"
	"go/types"

	"runtime.link/zgo/internal/source"
)

func (zig Target) println(expr source.FunctionCall) error {
	fmt.Fprintf(zig, "go.println(")
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
	fmt.Fprintf(zig, "\"%s\", .{", format)
	for i, arg := range expr.Arguments {
		if i > 0 {
			fmt.Fprintf(zig, ", ")
		}
		if err := zig.Expression(arg); err != nil {
			return err
		}
	}
	fmt.Fprintf(zig, "})")
	return nil
}

func (zig Target) new(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return expr.Errorf("new expects exactly one argument, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(zig, "go.new(goto, %s)", zig.TypeOf(expr.Arguments[0].TypeAndValue().Type))
	return nil
}

func (zig Target) make(expr source.FunctionCall) error {
	switch typ := expr.Arguments[0].TypeAndValue().Type.(type) {
	case *types.Slice:
		switch len(expr.Arguments) {
		case 2, 3:
		default:
			return expr.Errorf("make expects two or three arguments, got %d", len(expr.Arguments))
		}
		fmt.Fprintf(zig, "go.slice(%s).make(goto,",
			zig.TypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
		if err := zig.Expression(expr.Arguments[1]); err != nil {
			return err
		}
		fmt.Fprintf(zig, ",")
		if len(expr.Arguments) == 3 {
			if err := zig.Expression(expr.Arguments[2]); err != nil {
				return err
			}
		} else {
			if err := zig.Expression(expr.Arguments[1]); err != nil {
				return err
			}
		}
		fmt.Fprintf(zig, ")")
		return nil
	case *types.Chan:
		switch len(expr.Arguments) {
		case 1, 2:
		default:
			return expr.Errorf("make expects one or two arguments, got %d", len(expr.Arguments))
		}
		fmt.Fprintf(zig, "go.chan(%s).make(goto,", zig.TypeOf(typ.Elem()))
		if len(expr.Arguments) == 2 {
			if err := zig.Expression(expr.Arguments[1]); err != nil {
				return err
			}
		} else {
			fmt.Fprintf(zig, "0")
		}
		fmt.Fprintf(zig, ")")
		return nil
	case *types.Map:
		if len(expr.Arguments) != 1 {
			return expr.Errorf("make expects exactly one argument, got %d", len(expr.Arguments))
		}
		if typ.Key().String() == "string" {
			fmt.Fprintf(zig, "go.smap(%s).make(goto, 0)", zig.TypeOf(typ.Elem()))
		} else {
			fmt.Fprintf(zig, "go.map(%s, %s).make(goto, 0)", zig.TypeOf(typ.Key()), zig.TypeOf(typ.Elem()))
		}
		return nil
	default:
		return fmt.Errorf("unsupported type %T", expr.Arguments[0].TypeAndValue().Type)
	}
}

func (zig Target) append(expr source.FunctionCall) error {
	if len(expr.Arguments) != 2 {
		return expr.Errorf("append expects exactly two arguments, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(zig, "go.append(goto, %s, ", zig.TypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
	if err := zig.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(zig, ", ")
	if err := zig.Expression(expr.Arguments[1]); err != nil {
		return err
	}
	fmt.Fprintf(zig, ")")
	return nil
}

func (zig Target) copy(expr source.FunctionCall) error {
	if len(expr.Arguments) != 2 {
		return fmt.Errorf("copy expects exactly two arguments, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(zig, "go.copy(%s,", zig.TypeOf(expr.Arguments[0].TypeAndValue().Type.(*types.Slice).Elem()))
	if err := zig.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(zig, ", ")
	if err := zig.Expression(expr.Arguments[1]); err != nil {
		return err
	}
	fmt.Fprintf(zig, ")")
	return nil
}

func (zig Target) clear(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("clear expects exactly one argument, got %d", len(expr.Arguments))
	}
	if err := zig.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(zig, ".clear()")
	return nil
}

func (zig Target) len(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("len expects exactly one argument, got %d", len(expr.Arguments))
	}
	if err := zig.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(zig, ".len()")
	return nil
}

func (zig Target) cap(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("cap expects exactly one argument, got %d", len(expr.Arguments))
	}
	if err := zig.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(zig, ".cap()")
	return nil
}

func (zig Target) panic(expr source.FunctionCall) error {
	if len(expr.Arguments) != 1 {
		return fmt.Errorf("panic expects exactly one argument, got %d", len(expr.Arguments))
	}
	fmt.Fprintf(zig, "@panic(")
	if err := zig.Expression(expr.Arguments[0]); err != nil {
		return err
	}
	fmt.Fprintf(zig, ")")
	return nil
}
