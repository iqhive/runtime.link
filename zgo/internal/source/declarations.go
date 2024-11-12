package source

import (
	"go/token"

	"runtime.link/xyz"
)

type Declaration xyz.Tagged[Node, struct {
	Bad xyz.Case[Declaration, Bad]

	Function xyz.Case[Declaration, DeclarationFunction]
	Group    xyz.Case[Declaration, DeclarationGroup]
}]

var Declarations = xyz.AccessorFor(Declaration.Values)

func (decl Declaration) sources() Location {
	value, _ := decl.Get()
	return value.sources()
}

type DeclarationFunction struct {
	Location

	Documentation xyz.Maybe[CommentGroup]
	Receiver      xyz.Maybe[FieldList]

	Package string

	Name Identifier
	Type TypeFunction
	Body xyz.Maybe[StatementBlock]

	// Test is true when the function is a test function, within a test package.
	Test bool
}

type DeclarationGroup struct {
	Location

	Documentation  xyz.Maybe[CommentGroup]
	Token          WithLocation[token.Token]
	Opening        Location
	Specifications []Specification // FIXME
	Closing        Location
}
