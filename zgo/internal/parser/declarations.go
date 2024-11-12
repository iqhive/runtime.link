package parser

import (
	"go/ast"
	"go/token"
	"reflect"
	"strings"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func loadDeclaration(pkg *source.Package, node ast.Decl, top bool) source.Declaration {
	switch decl := node.(type) {
	case *ast.BadDecl:
		return source.Declarations.Bad.New(loadBad(pkg, decl, decl.From, decl.To))
	case *ast.FuncDecl:
		return source.Declarations.Function.New(loadDeclarationFunction(pkg, decl))
	case *ast.GenDecl:
		return source.Declarations.Group.New(loadDeclarationGroup(pkg, decl, top))
	default:
		panic("unexpected declaration type " + reflect.TypeOf(decl).String())
	}
}

func loadDeclarationFunction(pkg *source.Package, in *ast.FuncDecl) source.DeclarationFunction {
	var out source.DeclarationFunction
	out.Location = locationRangeIn(pkg, in.Pos(), in.End())
	if in.Doc != nil {
		out.Documentation = xyz.New(loadCommentGroup(pkg, in.Doc))
	}
	if in.Recv != nil {
		out.Receiver = xyz.New(loadFieldList(pkg, in.Recv))
	}
	out.Name = loadIdentifier(pkg, in.Name)
	out.Type = loadTypeFunction(pkg, in.Type)
	if in.Body != nil {
		out.Body = xyz.New(loadStatementBlock(pkg, in.Body))
	}
	out.Package = pkg.Name
	if pkg.Test &&
		strings.HasPrefix(out.Name.String, "Test") &&
		len(out.Type.Arguments.Fields) == 1 &&
		out.Type.Arguments.Fields[0].Type.TypeAndValue().Type.String() == "*testing.T" {
		out.Test = true
	}
	return out
}

func loadDeclarationGroup(pkg *source.Package, in *ast.GenDecl, top bool) source.DeclarationGroup {
	var out source.DeclarationGroup
	out.Location = locationRangeIn(pkg, in.Pos(), in.End())
	if in.Doc != nil {
		out.Documentation = xyz.New(loadCommentGroup(pkg, in.Doc))
	}
	out.Token = source.WithLocation[token.Token]{Value: in.Tok, SourceLocation: locationIn(pkg, in.TokPos)}
	out.Opening = locationIn(pkg, in.Lparen)
	for _, spec := range in.Specs {
		out.Specifications = append(out.Specifications, loadSpecification(pkg, spec, in.Tok == token.CONST, top))
	}
	out.Closing = locationIn(pkg, in.Rparen)
	return out
}
