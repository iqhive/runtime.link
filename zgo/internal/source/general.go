package source

import (
	"fmt"
	"go/token"
	"go/types"

	"runtime.link/xyz"
)

type File struct {
	MinimumGoVersion string

	Documentation xyz.Maybe[CommentGroup]
	Keyword       Location
	Name          Identifier
	Declarations  []Declaration
	FileFrom      Location
	FileUpto      Location
	Imports       []SpecificationImport
	Unresolved    []Identifier
	Comments      []CommentGroup
}

type Location struct {
	FileSet *token.FileSet
	Open    token.Pos
	Shut    token.Pos
}

func (loc Location) sources() Location { return loc }

func (loc Location) String() string {
	return loc.FileSet.Position(loc.Open).String()
}

type Node interface {
	sources() Location
}

type WithLocation[T any] struct {
	Value          T
	SourceLocation Location
}

type Bad Location

type Parenthesized struct {
	Typed

	Location

	Opening Location
	X       Expression
	Closing Location
}

type Selection struct {
	Typed
	Location
	X         Expression
	Selection Identifier

	Path []string
}

type Star struct {
	Typed
	Location
	WithLocation[Expression]
}

type Comment struct {
	Location

	Slash Location
	Text  string
}

type CommentGroup struct {
	Location
	List []Comment
}

type Field struct {
	Location

	Documentation xyz.Maybe[CommentGroup]
	Names         xyz.Maybe[[]Identifier]
	Type          Type
	Tag           xyz.Maybe[Constant]
	Comment       xyz.Maybe[CommentGroup]
}

type FieldList struct {
	Location

	Opening Location
	Fields  []Field
	Closing Location
}

type Identifier struct {
	Typed

	Location

	String string

	Method bool // identifier is a method

	Shadow int // number of shadowed identifiers

	Mutable bool // mutability analysis result
	Escapes bool // escape analysis result
	Package bool // identifier is global to the package and not defined within a sub-scope.

	IsPackage bool
}

type Package struct {
	types.Info

	Name  string
	Test  bool
	Files []File

	FileSet *token.FileSet
}

func (location Location) Errorf(format string, args ...interface{}) error {
	return fmt.Errorf(location.String()+": "+format, args...)
}

type DataComposite struct {
	Typed

	Location

	Type       xyz.Maybe[Type]
	OpenBrace  Location
	Elements   []Expression
	CloseBrace Location
	Incomplete bool
}

type Constant struct {
	Typed

	Location

	WithLocation[string]
	Kind token.Token
}
