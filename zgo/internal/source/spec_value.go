package source

import (
	"fmt"
	"go/ast"
	"io"

	"runtime.link/xyz"
)

type SpecificationValue struct {
	Documentation xyz.Maybe[CommentGroup]
	Names         []Identifier
	Type          xyz.Maybe[Type]
	Values        []Expression
	Comment       xyz.Maybe[CommentGroup]
}

func (pkg *Package) loadSpecificationValue(in *ast.ValueSpec) SpecificationValue {
	var out SpecificationValue
	if in.Doc != nil {
		out.Documentation = xyz.New(pkg.loadCommentGroup(in.Doc))
	}
	for _, name := range in.Names {
		out.Names = append(out.Names, pkg.loadIdentifier(name))
	}
	if in.Type != nil {
		out.Type = xyz.New(pkg.loadType(in.Type))
	}
	for _, value := range in.Values {
		out.Values = append(out.Values, pkg.loadExpression(value))
	}
	if in.Comment != nil {
		out.Comment = xyz.New(pkg.loadCommentGroup(in.Comment))
	}
	return out
}

func (spec SpecificationValue) compile(w io.Writer) error {
	for i, name := range spec.Names {
		value := spec.Values[i]
		if name.Name.Value == "_" {
			fmt.Fprintf(w, "_ = ")
			if err := value.compile(w); err != nil {
				return err
			}
		} else {
			fmt.Fprintf(w, "var %s: %s = ", name.Name.Value, zigTypeOf(value.TypeAndValue().Type))
			if err := value.compile(w); err != nil {
				return err
			}
			fmt.Fprintf(w, "; %s=%s", name.Name.Value, name.Name.Value)
		}
	}
	return nil
}
