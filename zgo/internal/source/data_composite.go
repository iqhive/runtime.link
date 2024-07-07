package source

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"

	"runtime.link/xyz"
)

type DataComposite struct {
	typed

	Location

	Type       xyz.Maybe[Type]
	OpenBrace  Location
	Elements   []Expression
	CloseBrace Location
	Incomplete bool
}

func (pkg *Package) loadDataComposite(in *ast.CompositeLit) DataComposite {
	var out DataComposite
	out.Location = pkg.locations(in.Pos(), in.End())
	out.typed = pkg.typed(in)
	if ctype := in.Type; ctype != nil {
		out.Type = xyz.New(pkg.loadType(ctype))
	}
	out.OpenBrace = pkg.location(in.Lbrace)
	for _, expr := range in.Elts {
		out.Elements = append(out.Elements, pkg.loadExpression(expr))
	}
	out.CloseBrace = pkg.location(in.Rbrace)
	out.Incomplete = in.Incomplete
	return out
}

func (data DataComposite) compile(w io.Writer, tabs int) error {
	dtype, ok := data.Type.Get()
	if !ok {
		return data.Errorf("composite literal missing type")
	}
	fmt.Fprintf(w, "%s", dtype.ZigType())
	switch typ := dtype.TypeAndValue().Type.Underlying().(type) {
	case *types.Array:
		fmt.Fprintf(w, "{")
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			if err := elem.compile(w, tabs); err != nil {
				return err
			}
		}
		fmt.Fprintf(w, "}")
		return nil
	case *types.Slice:
		fmt.Fprintf(w, ".literal(goto, %d, .", len(data.Elements))
		fmt.Fprintf(w, "{")
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			if err := elem.compile(w, tabs); err != nil {
				return err
			}
		}
		fmt.Fprintf(w, "})")
		return nil
	case *types.Map:
		fmt.Fprintf(w, ".literal(goto, %d, .", len(data.Elements))
		fmt.Fprintf(w, "{")
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			pair := Expressions.KeyValue.Get(elem)
			fmt.Fprintf(w, ".{")
			if err := pair.Key.compile(w, tabs); err != nil {
				return err
			}
			fmt.Fprintf(w, ", ")
			if err := pair.Value.compile(w, tabs); err != nil {
				return err
			}
			fmt.Fprintf(w, "}")
		}
		fmt.Fprintf(w, "})")
		return nil
	case *types.Struct:
		fmt.Fprintf(w, "{")
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			switch xyz.ValueOf(elem) {
			case Expressions.KeyValue:
				if err := elem.compile(w, tabs); err != nil {
					return err
				}
			default:
				field := typ.Field(i)
				fmt.Fprintf(w, ".%s = ", field.Name())
				if err := elem.compile(w, tabs); err != nil {
					return err
				}
			}
		}
		fmt.Fprintf(w, "}")
		return nil
	default:
		return data.Errorf("unexpected composite type: " + typ.String())
	}
}
