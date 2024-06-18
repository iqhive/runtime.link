package source

import (
	"go/ast"

	"runtime.link/xyz"
)

type Field struct {
	Location

	Documentation xyz.Maybe[CommentGroup]
	Names         xyz.Maybe[[]Identifier]
	Type          Type
	Tag           xyz.Maybe[Constant]
	Comment       xyz.Maybe[CommentGroup]
}

func (pkg *Package) loadField(in *ast.Field) Field {
	var out Field
	out.Location = pkg.location(in.Pos())
	if in.Doc != nil {
		out.Documentation = xyz.New(pkg.loadCommentGroup(in.Doc))
	}
	if in.Names != nil {
		var names []Identifier
		for _, name := range in.Names {
			names = append(names, pkg.loadIdentifier(name))
		}
		out.Names = xyz.New(names)
	}
	out.Type = pkg.loadType(in.Type)
	if in.Tag != nil {
		out.Tag = xyz.New(pkg.loadConstant(in.Tag))
	}
	if in.Comment != nil {
		out.Comment = xyz.New(pkg.loadCommentGroup(in.Comment))
	}
	return out
}

type FieldList struct {
	Location

	Opening Location
	Fields  []Field
	Closing Location
}

func (pkg *Package) loadFieldList(in *ast.FieldList) FieldList {
	var out FieldList
	out.Location = pkg.locations(in.Pos(), in.End())
	if in != nil {
		out.Opening = pkg.location(in.Opening)
		for _, field := range in.List {
			out.Fields = append(out.Fields, pkg.loadField(field))
		}
		out.Closing = pkg.location(in.Closing)
	}
	return out
}
