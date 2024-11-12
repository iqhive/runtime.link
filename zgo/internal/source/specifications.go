package source

import (
	"runtime.link/xyz"
)

type Specification xyz.Tagged[Node, struct {
	Bad xyz.Case[Specification, Bad]

	Type   xyz.Case[Specification, SpecificationType]
	Value  xyz.Case[Specification, SpecificationValue]
	Import xyz.Case[Specification, SpecificationImport]
}]

var Specifications = xyz.AccessorFor(Specification.Values)

func (spec Specification) sources() Location {
	value, _ := spec.Get()
	return value.sources()
}

type SpecificationImport struct {
	Location

	Documentation xyz.Maybe[CommentGroup]
	Name          xyz.Maybe[Identifier]
	Path          Constant
	Comment       xyz.Maybe[CommentGroup]
	End           Location
}

type SpecificationType struct {
	Location

	Typed

	Documentation  xyz.Maybe[CommentGroup]
	Name           Identifier
	TypeParameters xyz.Maybe[FieldList]
	Assign         Location
	Type           Type
	Package        string

	// PackageLevelScope means the type is defined
	// at the package level instead of inside a function.
	PackageLevelScope bool
}

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
