package source

import (
	"go/ast"
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
	if dtype, ok := data.Type.Get(); ok {
		if err := dtype.compile(w, tabs); err != nil {
			return err
		}
	}
	for _, elem := range data.Elements {
		if err := elem.compile(w, tabs); err != nil {
			return err
		}
	}
	return nil
}
