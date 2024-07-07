package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"strings"
)

type Location struct {
	fset *token.FileSet
	open token.Pos
	shut token.Pos
}

func (loc Location) sources() Location {
	return loc
}

func (loc Location) String() string {
	return loc.fset.Position(loc.open).String()
}

type Node interface {
	sources() Location
	compile(io.Writer, int) error
}

func toString(node Node) string {
	var buf strings.Builder
	node.compile(&buf, 0)
	return buf.String()
}

type WithLocation[T any] struct {
	Value          T
	SourceLocation Location
}

type Bad Location

func (pkg *Package) loadBad(node ast.Node, from, upto token.Pos) Bad {
	return Bad(pkg.locations(from, upto))
}

type Parenthesized struct {
	typed

	Location

	Opening Location
	X       Expression
	Closing Location
}

func (pkg *Package) loadParenthesized(in *ast.ParenExpr) Parenthesized {
	return Parenthesized{
		Location: pkg.locations(in.Pos(), in.End()),
		typed:    pkg.typed(in),
		Opening:  pkg.location(in.Lparen),
		X:        pkg.loadExpression(in.X),
		Closing:  pkg.location(in.Rparen),
	}
}

func (par Parenthesized) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "(")
	if err := par.X.compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, ")")
	return nil
}

type Selection struct {
	typed
	Location
	X         Expression
	Selection Identifier

	Path []string
}

func (pkg *Package) loadSelection(in *ast.SelectorExpr) Selection {
	sel := Selection{
		Location:  pkg.locations(in.Pos(), in.End()),
		typed:     pkg.typed(in),
		X:         pkg.loadExpression(in.X),
		Selection: pkg.loadIdentifier(in.Sel),
	}
	meta, ok := pkg.Selections[in]
	if ok && len(meta.Index()) > 1 && meta.Kind() == types.FieldVal {
		ptype := sel.X.TypeAndValue().Type.Underlying()
		for index := range meta.Index()[1:] {
			for {
				ptr, ok := ptype.(*types.Pointer)
				if !ok {
					break
				}
				ptype = ptr.Elem().Underlying()
			}
			rtype := ptype.(*types.Struct)
			field := rtype.Field(index)
			sel.Path = append(sel.Path, field.Name())
			ptype = field.Type().Underlying()
		}
	}
	return sel
}

func (sel Selection) compile(w io.Writer, tabs int) error {
	if err := sel.X.compile(w, tabs); err != nil {
		return err
	}
	for _, elem := range sel.Path {
		fmt.Fprintf(w, ".%s", elem)
	}
	fmt.Fprintf(w, ".")
	return sel.Selection.compile(w, tabs)
}

type Star struct {
	typed
	Location
	WithLocation[Expression]
}

func (pkg *Package) loadStar(in *ast.StarExpr) Star {
	return Star{
		Location: pkg.locations(in.Pos(), in.End()),
		typed:    pkg.typed(in),
		WithLocation: WithLocation[Expression]{
			Value:          pkg.loadExpression(in.X),
			SourceLocation: pkg.location(in.Star),
		},
	}
}

func (star Star) compile(w io.Writer, tabs int) error {
	if err := star.Value.compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, ".get()")
	return nil
}
