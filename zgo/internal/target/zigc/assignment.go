package zigc

import (
	"fmt"
	"go/token"
	"go/types"
	"strings"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func (zig Target) StatementAssignment(stmt source.StatementAssignment) error {
	if stmt.Token.Value == token.DEFINE {
		var names []source.Identifier
		for i, variable := range stmt.Variables {
			switch xyz.ValueOf(variable) {
			case source.Expressions.Identifier:
				ident := source.Expressions.Identifier.Get(variable)
				if ident.String == "_" {
					fmt.Fprintf(zig, "go.use(")
					if err := zig.Expression(stmt.Values[i]); err != nil {
						return err
					}
					fmt.Fprintf(zig, ")")
					break
				}
				names = append(names, ident)
			default:
				return stmt.Location.Errorf("unsupported variable assignment")
			}
		}
		zig.Tabs = -zig.Tabs
		return zig.SpecificationValue(source.SpecificationValue{
			Names:  names,
			Values: stmt.Values,
		})
	}
	for i, variable := range stmt.Variables {
		switch xyz.ValueOf(variable) {
		case source.Expressions.Star:
			expr := source.Expressions.Star.Get(variable)
			if err := zig.Expression(expr.Value); err != nil {
				return err
			}
			fmt.Fprintf(zig, ".set(")
			if err := zig.Expression(stmt.Values[i]); err != nil {
				return err
			}
			fmt.Fprintf(zig, ")")
		case source.Expressions.Index:
			expr := source.Expressions.Index.Get(variable)
			if mtype, ok := expr.X.TypeAndValue().Type.(*types.Map); ok {
				if mtype.Key().String() == "string" {
					if err := zig.Expression(expr.X); err != nil {
						return err
					}
					fmt.Fprintf(zig, ".set(goto,")
					if err := zig.Expression(expr.Index); err != nil {
						return err
					}
					fmt.Fprintf(zig, ", ")
					if err := zig.Expression(stmt.Values[i]); err != nil {
						return err
					}
					fmt.Fprintf(zig, ")")
					return nil
				}
				fmt.Fprintf(zig, "go.runtime.map_set(%s, %s,", zig.TypeOf(mtype.Key()), zig.TypeOf(mtype.Elem()))
				if err := zig.Expression(expr.X); err != nil {
					return err
				}
				fmt.Fprintf(zig, ", ")
				if err := zig.Expression(expr.Index); err != nil {
					return err
				}
				fmt.Fprintf(zig, ", ")
				if err := zig.Expression(stmt.Values[i]); err != nil {
					return err
				}
				fmt.Fprintf(zig, ")")
				return nil
			}
			fallthrough
		default:
			if xyz.ValueOf(variable) == source.Expressions.Identifier {
				ident := source.Expressions.Identifier.Get(variable)
				if ident.String == "_" {
					fmt.Fprintf(zig, "go.use(")
					if err := zig.Expression(stmt.Values[i]); err != nil {
						return err
					}
					fmt.Fprintf(zig, ")")
					break
				}
			}
			if err := zig.Expression(variable); err != nil {
				return err
			}
			fmt.Fprintf(zig, " %s ", stmt.Token.Value)
			switch variable.TypeAndValue().Type.(type) {
			case *types.Interface:
				if strings.HasPrefix(zig.TypeOf(stmt.Values[i].TypeAndValue().Type), "go.pointer(") {
					fmt.Fprintf(zig, "go.any{.rtype=%s,.value=", zig.ReflectTypeOf(stmt.Values[i].TypeAndValue().Type))
					if err := zig.Expression(stmt.Values[i]); err != nil {
						return nil
					}
					fmt.Fprintf(zig, ".address}")
				} else {
					fmt.Fprintf(zig, "go.any.make(%s, goto, %s, ", zig.TypeOf(stmt.Values[i].TypeAndValue().Type), zig.ReflectTypeOf(stmt.Values[i].TypeAndValue().Type))
					if err := zig.Expression(stmt.Values[i]); err != nil {
						return err
					}
					fmt.Fprintf(zig, ")")
				}
			default:
				if err := zig.Expression(stmt.Values[i]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
