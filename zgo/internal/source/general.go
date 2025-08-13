package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"runtime.link/xyz"
)

// File within a Go [Package].
type File struct {
	Location

	MinimumGoVersion string

	Documentation  xyz.Maybe[CommentGroup]
	PackageKeyword Location
	PackageName    ImportedPackage
	Imports        []Import
	Definitions    []Definition // functions, variables and types.
	Unresolved     []Identifier
	Comments       []CommentGroup
}

type EscapeInformation struct {
	Block       func() EscapeFeasibility // true if the variable escapes the block it was defined in.
	Function    func() EscapeFeasibility // true if the variable escapes the function it was defined in.
	Goroutine   func() EscapeFeasibility // true if the variable escapes the goroutine it was defined in.
	Containment func() EscapeFeasibility // true if the variable escapes into the global scope.
}

type EscapeFeasibility struct {
	Possible bool
	Together []Identifier // escape is only-possible when at least one of these identifiers can escape.
	WithBits []Expression // escape is only-possible when these escape bits resolve to one (func type).
}

type Import struct {
	Location

	Rename  xyz.Maybe[ImportedPackage]
	Path    Literal
	Comment xyz.Maybe[CommentGroup]
	End     Location
}

// Location within a set of files.
type Location struct {
	FileSet *token.FileSet
	Node    ast.Node
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

func LocationOf(node Node) Location {
	if node == nil {
		return Location{}
	}
	return node.sources()
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
	Selection Expression

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
	Names         xyz.Maybe[[]DefinedVariable]
	Type          Type
	Tag           xyz.Maybe[Literal]
	Comment       xyz.Maybe[CommentGroup]
}

type FieldList struct {
	Location

	Opening Location
	Fields  []Field
	Closing Location
}

type Unique interface {
	types.Object
}

type Identifier struct {
	Typed
	Location

	Unique Unique

	String string

	Method bool // identifier is a method

	Shadow int // number of shadowed identifiers

	Mutable bool              // mutability analysis result
	Escapes EscapeInformation // escape analysis result
	Package bool              // identifier is global to the package and not defined within a sub-scope.

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

type Literal struct {
	Typed

	Location

	WithLocation[string]
	Kind token.Token
}
