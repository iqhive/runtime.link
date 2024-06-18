package source

import (
	"go/ast"
	"io"
	"reflect"

	"runtime.link/xyz"
)

type Declaration xyz.Switch[Node, struct {
	Bad xyz.Case[Declaration, Bad]

	Function xyz.Case[Declaration, DeclarationFunction]
	Group    xyz.Case[Declaration, DeclarationGroup]
}]

var Declarations = xyz.AccessorFor(Declaration.Values)

func (pkg *Package) loadDeclaration(node ast.Decl, top bool) Declaration {
	switch decl := node.(type) {
	case *ast.BadDecl:
		return Declarations.Bad.New(pkg.loadBad(decl, decl.From, decl.To))
	case *ast.FuncDecl:
		return Declarations.Function.New(pkg.loadDeclarationFunction(decl))
	case *ast.GenDecl:
		return Declarations.Group.New(pkg.loadDeclarationGroup(decl, top))
	default:
		panic("unexpected declaration type " + reflect.TypeOf(decl).String())
	}
}

func (decl Declaration) sources() Location {
	value, _ := decl.Get()
	return value.sources()
}

func (decl Declaration) compile(w io.Writer, tabs int) error {
	value, _ := decl.Get()
	return value.compile(w, tabs)
}
