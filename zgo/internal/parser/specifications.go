package parser

import (
	"go/ast"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func loadImport(pkg *source.Package, in *ast.ImportSpec) source.Import {
	var out source.Import
	out.Location = locationRangeIn(pkg, in.Pos(), in.End())
	if in.Name != nil {
		out.Rename = xyz.New(source.ImportedPackage(loadIdentifier(pkg, in.Name)))
	}
	out.Path = loadConstant(pkg, in.Path)
	if in.Comment != nil {
		out.Comment = xyz.New(loadCommentGroup(pkg, in.Comment))
	}
	out.End = locationIn(pkg, in.End())
	return out
}
