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
	out.typed = typed{pkg.Types[in]}
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
	var call bool

	dtype, ok := data.Type.Get()
	if ok {
		fmt.Fprintf(w, "%s", zigTypeOf(dtype.TypeAndValue().Type))
		switch dtype.TypeAndValue().Type.(type) {
		case *types.Slice:
			fmt.Fprintf(w, ".literal(goto, %d, .", len(data.Elements))
			call = true
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
		}
	}

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
	if call {
		fmt.Fprintf(w, ")")
	}
	return nil
}
