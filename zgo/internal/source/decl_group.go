package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"

	"runtime.link/xyz"
)

type DeclarationGroup struct {
	Documentation  xyz.Maybe[CommentGroup]
	Token          WithLocation[token.Token]
	Opening        Location
	Specifications []Specification // FIXME
	Closing        Location
}

func (pkg *Package) loadDeclarationGroup(in *ast.GenDecl) DeclarationGroup {
	var out DeclarationGroup
	if in.Doc != nil {
		out.Documentation = xyz.New(pkg.loadCommentGroup(in.Doc))
	}
	out.Token = WithLocation[token.Token]{Value: in.Tok, SourceLocation: Location(in.TokPos)}
	out.Opening = Location(in.Lparen)
	for _, spec := range in.Specs {
		out.Specifications = append(out.Specifications, pkg.loadSpecification(spec))
	}
	out.Closing = Location(in.Rparen)
	return out
}

func (decl DeclarationGroup) compile(w io.Writer) error {
	for i, spec := range decl.Specifications {
		if err := spec.compile(w); err != nil {
			return err
		}
		if i < len(decl.Specifications)-1 {
			fmt.Fprintf(w, ";\n")
		}
	}
	return nil
}
