package source

import (
	"fmt"
	"go/ast"
	"go/token"
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
		typed:    typed{pkg.Types[in]},
		Opening:  pkg.location(in.Lparen),
		X:        pkg.loadExpression(in.X),
		Closing:  pkg.location(in.Rparen),
	}
}

type Selection struct {
	typed
	Location
	X         Expression
	Selection Identifier
}

func (pkg *Package) loadSelection(in *ast.SelectorExpr) Selection {
	return Selection{
		Location:  pkg.locations(in.Pos(), in.End()),
		typed:     typed{pkg.Types[in]},
		X:         pkg.loadExpression(in.X),
		Selection: pkg.loadIdentifier(in.Sel),
	}
}

func (sel Selection) compile(w io.Writer, tabs int) error {
	if err := sel.X.compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, ".%s", sel.Selection.Name.Value)
	return nil
}

type Star struct {
	typed
	Location
	WithLocation[Expression]
}

func (pkg *Package) loadStar(in *ast.StarExpr) Star {
	return Star{
		Location: pkg.locations(in.Pos(), in.End()),
		typed:    typed{pkg.Types[in]},
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
