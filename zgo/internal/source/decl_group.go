package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"strings"

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

func (pkg *Package) loadDeclarationGroup(in *ast.GenDecl) DeclarationGroup {
	var out DeclarationGroup
	out.Location = pkg.locations(in.Pos(), in.End())
	if in.Doc != nil {
		out.Documentation = xyz.New(pkg.loadCommentGroup(in.Doc))
	}
	out.Token = WithLocation[token.Token]{Value: in.Tok, SourceLocation: pkg.location(in.TokPos)}
	out.Opening = pkg.location(in.Lparen)
	for _, spec := range in.Specs {
		out.Specifications = append(out.Specifications, pkg.loadSpecification(spec, in.Tok == token.CONST))
	}
	out.Closing = pkg.location(in.Rparen)
	return out
}

func (decl DeclarationGroup) compile(w io.Writer, tabs int) error {
	for i, spec := range decl.Specifications {
		if err := spec.compile(w, tabs); err != nil {
			return err
		}
		if i < len(decl.Specifications)-1 {
			fmt.Fprintf(w, ";")
			fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
		}
	}
	return nil
}
