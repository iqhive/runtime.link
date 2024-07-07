package source

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"strings"

	"runtime.link/xyz"
)

type SpecificationValue struct {
	Location
	Documentation     xyz.Maybe[CommentGroup]
	Names             []Identifier
	Type              xyz.Maybe[Type]
	Values            []Expression
	Comment           xyz.Maybe[CommentGroup]
	Const             bool
	PackageLevelScope bool
}

func (pkg *Package) loadSpecificationValue(in *ast.ValueSpec, constant bool, top bool) SpecificationValue {
	var out SpecificationValue
	out.Const = constant
	out.PackageLevelScope = top
	out.Location = pkg.locations(in.Pos(), in.End())
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

func (spec SpecificationValue) compile(w io.Writer, tabs int) error {
	for i, name := range spec.Names {
		if tabs > 0 {
			fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
		}
		var value func(io.Writer, int) error
		var rtype types.Type
		var ztype string
		vtype, ok := spec.Type.Get()
		if !ok && len(spec.Values) == 0 {
			return fmt.Errorf("missing type for value %s", name.String())
		}
		if ok {
			rtype = vtype.TypeAndValue().Type
			ztype = vtype.ZigType()

		} else if len(spec.Values) > 0 {
			rtype = spec.Values[i].TypeAndValue().Type
			ztype = spec.Values[i].ZigType()
		}
		if len(spec.Values) == 0 {
			value = func(w io.Writer, tabs int) error {
				if ztype[0] == '*' {
					fmt.Fprintf(w, "null")
					return nil
				}
				fmt.Fprintf(w, "go.zero(%s)", ztype)
				return nil
			}
		} else {
			value = spec.Values[i].compile

			_, isInterface := rtype.Underlying().(*types.Interface)
			if isInterface {
				value = ExpressionCall{
					Location:  spec.Location,
					Function:  Expressions.Type.As(vtype),
					Arguments: []Expression{spec.Values[i]},
				}.compile
			}
		}
		if name.String() == "_" {
			fmt.Fprintf(w, "_ = ")
			if err := value(w, tabs); err != nil {
				return err
			}
		} else {
			if spec.Const {
				fmt.Fprintf(w, "const ")
			} else {
				fmt.Fprintf(w, "var ")
			}
			if err := name.compile(w, tabs); err != nil {
				return err
			}
			fmt.Fprintf(w, ": %s = ", ztype)
			if err := value(w, tabs); err != nil {
				return err
			}
			if !spec.Const && !spec.PackageLevelScope {
				fmt.Fprintf(w, ";")
				if err := name.compile(w, tabs); err != nil {
					return err
				}
				fmt.Fprintf(w, "=")
				if err := name.compile(w, tabs); err != nil {
					return err
				}
			}
		}
		if tabs > 0 || spec.PackageLevelScope {
			fmt.Fprintf(w, ";")
		}
	}
	return nil
}
