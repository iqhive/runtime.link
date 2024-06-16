package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"

	"runtime.link/xyz"
)

type Node interface {
	compile(io.Writer) error
}

type Location token.Pos

type WithLocation[T any] struct {
	Value          T
	SourceLocation Location
}

type Bad struct {
	From, Upto Location
}

func (pkg *Package) loadBad(node ast.Node, from, upto token.Pos) Bad {
	return Bad{
		From: Location(from),
		Upto: Location(upto),
	}
}

type SwitchCaseClause struct {
	Keyword     Location
	Expressions []Expression
	Colon       Location
	Body        []Statement
}

func (pkg *Package) loadSwitchCaseClause(in *ast.CaseClause) SwitchCaseClause {
	var out SwitchCaseClause
	out.Keyword = Location(in.Case)
	for _, expr := range in.List {
		out.Expressions = append(out.Expressions, pkg.loadExpression(expr))
	}
	out.Colon = Location(in.Colon)
	for _, stmt := range in.Body {
		out.Body = append(out.Body, pkg.loadStatement(stmt))
	}
	return out
}

type SelectCaseClause struct {
	Keyword   Location
	Statement Statement
	Colon     Location
	Body      []Statement
}

func (pkg *Package) loadSelectCaseClause(in *ast.CommClause) SelectCaseClause {
	var out SelectCaseClause
	out.Keyword = Location(in.Case)
	out.Statement = pkg.loadStatement(in.Comm)
	out.Colon = Location(in.Colon)
	for _, stmt := range in.Body {
		out.Body = append(out.Body, pkg.loadStatement(stmt))
	}
	return out
}

type Comment struct {
	Slash Location
	Text  string
}

func (pkg *Package) loadComment(in *ast.Comment) Comment {
	return Comment{
		Slash: Location(in.Slash),
		Text:  in.Text,
	}
}

type CommentGroup struct {
	List []Comment
}

func (pkg *Package) loadCommentGroup(in *ast.CommentGroup) CommentGroup {
	var out CommentGroup
	for _, comment := range in.List {
		out.List = append(out.List, pkg.loadComment(comment))
	}
	return out
}

type Field struct {
	Documentation xyz.Maybe[CommentGroup]
	Names         xyz.Maybe[[]Identifier]
	Type          Type
	Tag           xyz.Maybe[BasicLiteral]
	Comment       xyz.Maybe[CommentGroup]
}

func (pkg *Package) loadField(in *ast.Field) Field {
	var out Field
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
		out.Tag = xyz.New(pkg.loadBasicLiteral(in.Tag))
	}
	if in.Comment != nil {
		out.Comment = xyz.New(pkg.loadCommentGroup(in.Comment))
	}
	return out
}

type FieldList struct {
	Opening Location
	Fields  []Field
	Closing Location
}

func (pkg *Package) loadFieldList(in *ast.FieldList) FieldList {
	var out FieldList
	if in != nil {
		out.Opening = Location(in.Opening)
		for _, field := range in.List {
			out.Fields = append(out.Fields, pkg.loadField(field))
		}
		out.Closing = Location(in.Closing)
	}
	return out
}

type Parenthesized struct {
	typed

	Opening Location
	X       Expression
	Closing Location
}

func (pkg *Package) loadParenthesized(in *ast.ParenExpr) Parenthesized {
	return Parenthesized{
		typed:   typed{pkg.Types[in]},
		Opening: Location(in.Lparen),
		X:       pkg.loadExpression(in.X),
		Closing: Location(in.Rparen),
	}
}

type Selection struct {
	typed
	X         Expression
	Selection Identifier
}

func (pkg *Package) loadSelection(in *ast.SelectorExpr) Selection {
	return Selection{
		typed:     typed{pkg.Types[in]},
		X:         pkg.loadExpression(in.X),
		Selection: pkg.loadIdentifier(in.Sel),
	}
}

func (sel Selection) compile(w io.Writer) error {
	if err := sel.X.compile(w); err != nil {
		return err
	}
	fmt.Fprintf(w, ".%s", sel.Selection.Name.Value)
	return nil
}

type Star struct {
	typed
	WithLocation[Expression]
}

func (pkg *Package) loadStar(in *ast.StarExpr) Star {
	return Star{
		typed: typed{pkg.Types[in]},
		WithLocation: WithLocation[Expression]{
			Value:          pkg.loadExpression(in.X),
			SourceLocation: Location(in.Star),
		},
	}
}

func (star Star) compile(w io.Writer) error {
	if err := star.Value.compile(w); err != nil {
		return err
	}
	fmt.Fprintf(w, ".*")
	return nil
}
