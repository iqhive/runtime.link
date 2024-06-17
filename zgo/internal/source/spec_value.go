package source

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"

	"runtime.link/xyz"
)

type SpecificationValue struct {
	Documentation xyz.Maybe[CommentGroup]
	Names         []Identifier
	Type          xyz.Maybe[Type]
	Values        []Expression
	Comment       xyz.Maybe[CommentGroup]
	Const         bool
}

func (pkg *Package) loadSpecificationValue(in *ast.ValueSpec, constant bool) SpecificationValue {
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
		var value func(io.Writer) error
		var rtype types.Type
		if len(spec.Values) > 0 {
			value = spec.Values[i].compile
			rtype = spec.Values[i].TypeAndValue().Type
		} else {
			vtype, ok := spec.Type.Get()
			if !ok {
				return fmt.Errorf("missing type for value %s", name.Name.Value)
			}
			rtype = vtype.TypeAndValue().Type
			value = func(w io.Writer) error {
				ztype := zigTypeOf(rtype)
				if ztype[0] == '*' {
					fmt.Fprintf(w, "null")
					return nil
				}
				fmt.Fprintf(w, "std.mem.zeroes(%s)", ztype)
				return nil
			}
		}
		if name.Name.Value == "_" {
			fmt.Fprintf(w, "_ = ")
			if err := value(w); err != nil {
				return err
			}
		} else {
			if spec.Const {
				fmt.Fprintf(w, "const ")
			} else {
				fmt.Fprintf(w, "var ")
			}
			fmt.Fprintf(w, "%s: %s = ", name.Name.Value, zigTypeOf(rtype))
			if err := value(w); err != nil {
				return err
			}
			fmt.Fprintf(w, "; %s=%s", name.Name.Value, name.Name.Value)
		}
	}
	return nil
}
