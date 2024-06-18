package source

import "go/ast"

type Comment struct {
	Location

	Slash Location
	Text  string
}

func (pkg *Package) loadComment(in *ast.Comment) Comment {
	return Comment{
		Location: pkg.locations(in.Pos(), in.End()),
		Slash:    pkg.location(in.Slash),
		Text:     in.Text,
	}
}

type CommentGroup struct {
	Location
	List []Comment
}

func (pkg *Package) loadCommentGroup(in *ast.CommentGroup) CommentGroup {
	var out CommentGroup
	out.Location = pkg.locations(in.Pos(), in.End())
	for _, comment := range in.List {
		out.List = append(out.List, pkg.loadComment(comment))
	}
	return out
}
