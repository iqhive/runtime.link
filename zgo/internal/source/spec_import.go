package source

import (
	"fmt"
	"go/ast"
	"io"
	"path"
	"strconv"

	"runtime.link/xyz"
)

type SpecificationImport struct {
	Location

	Documentation xyz.Maybe[CommentGroup]
	Name          xyz.Maybe[Identifier]
	Path          Constant
	Comment       xyz.Maybe[CommentGroup]
	End           Location
}

func (pkg *Package) loadImport(in *ast.ImportSpec) SpecificationImport {
	var out SpecificationImport
	out.Location = pkg.locations(in.Pos(), in.End())
	if in.Doc != nil {
		out.Documentation = xyz.New(pkg.loadCommentGroup(in.Doc))
	}
	if in.Name != nil {
		out.Name = xyz.New(pkg.loadIdentifier(in.Name))
	}
	out.Path = pkg.loadConstant(in.Path)
	if in.Comment != nil {
		out.Comment = xyz.New(pkg.loadCommentGroup(in.Comment))
	}
	out.End = pkg.location(in.End())
	return out
}

func (spec SpecificationImport) compile(w io.Writer, tabs int) error {
	return nil
	path, _ := strconv.Unquote(path.Base(spec.Path.Value))
	rename, ok := spec.Name.Get()
	if ok {
		fmt.Fprintf(w, "const %s = ", rename.Name.Value)
	} else {

		fmt.Fprintf(w, "const %s = ", path)
	}
	fmt.Fprintf(w, "@import(%q);\n", path+".zig")
	return nil
}
