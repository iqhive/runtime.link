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

func (pkg *Package) loadDeclaration(node ast.Decl) Declaration {
	switch decl := node.(type) {
	case *ast.BadDecl:
		return Declarations.Bad.New(pkg.loadBad(decl, decl.From, decl.To))
	case *ast.FuncDecl:
		return Declarations.Function.New(pkg.loadDeclarationFunction(decl))
	case *ast.GenDecl:
		return Declarations.Group.New(pkg.loadDeclarationGroup(decl))
	default:
		panic("unexpected declaration type " + reflect.TypeOf(decl).String())
	}
}

func (decl Declaration) compile(w io.Writer) error {
	value, _ := decl.Get()
	return value.compile(w)
}

type SpecificationType struct {
	Documentation  xyz.Maybe[CommentGroup]
	Name           Identifier
	TypeParameters xyz.Maybe[FieldList]
	Assign         Location
	Type           Type
}

func (pkg *Package) loadSpecificationType(in *ast.TypeSpec) SpecificationType {
	var out SpecificationType
	if in.Doc != nil {
		out.Documentation = xyz.New(pkg.loadCommentGroup(in.Doc))
	}
	out.Name = pkg.loadIdentifier(in.Name)
	if in.TypeParams != nil {
		out.TypeParameters = xyz.New(pkg.loadFieldList(in.TypeParams))
	}
	out.Assign = Location(in.Assign)
	out.Type = pkg.loadType(in.Type)
	return out
}
