package source

import (
	"fmt"
	"go/token"
	"go/types"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/escape"
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

type Identifier struct {
	Typed

	Location

	String string

	Method bool // identifier is a method

	Shadow int // number of shadowed identifiers

	Mutable bool // mutability analysis result
	Package bool // identifier is global to the package and not defined within a sub-scope
	// EscapeInfo contains detailed escape analysis results
	EscapeInfo escape.Info
	// Escapes is maintained for backward compatibility, true if EscapeInfo.Kind != NoEscape
	Escapes bool

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
