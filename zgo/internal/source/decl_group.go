package source

import (
	"go/ast"
	"go/token"
	"io"

	"runtime.link/xyz"
)

type DeclarationGroup struct {
	Location

	Documentation  xyz.Maybe[CommentGroup]
	Token          WithLocation[token.Token]
	Opening        Location
	Specifications []Specification // FIXME
	Closing        Location
}

func (pkg *Package) loadDeclarationGroup(in *ast.GenDecl, top bool) DeclarationGroup {
	var out DeclarationGroup
	out.Location = pkg.locations(in.Pos(), in.End())
	if in.Doc != nil {
		out.Documentation = xyz.New(pkg.loadCommentGroup(in.Doc))
	}
	out.Token = WithLocation[token.Token]{Value: in.Tok, SourceLocation: pkg.location(in.TokPos)}
	out.Opening = pkg.location(in.Lparen)
	for _, spec := range in.Specs {
		out.Specifications = append(out.Specifications, pkg.loadSpecification(spec, in.Tok == token.CONST, top))
	}
	out.Closing = pkg.location(in.Rparen)
	return out
}

func (decl DeclarationGroup) compile(w io.Writer, tabs int) error {
	for _, spec := range decl.Specifications {
		if err := spec.compile(w, tabs); err != nil {
			return err
		}
	}
	return nil
}
