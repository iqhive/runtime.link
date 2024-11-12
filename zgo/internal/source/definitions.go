package source

import (
	"runtime.link/xyz"
)

// Definition within a [Package], either for a type, const, var or func.
type Definition xyz.Tagged[Node, struct {
	Invalid xyz.Case[Definition, Bad]

	Type xyz.Case[Definition, TypeDefinition]

	Constant xyz.Case[Definition, ConstantDefinition]
	Variable xyz.Case[Definition, VariableDefinition]
	Function xyz.Case[Definition, FunctionDefinition]
}]

// Definitions union.
var Definitions = xyz.AccessorFor(Definition.Values)

func (decl Definition) sources() Location {
	value, _ := decl.Get()
	return value.sources()
}

type FunctionDefinition struct {
	Location

	Documentation xyz.Maybe[CommentGroup]
	Receiver      xyz.Maybe[FieldList]

	Package string

	Name DefinedFunction
	Type TypeFunction
	Body xyz.Maybe[StatementBlock]

	IsTest bool // true when the function is a test function, within a test package.
}

type VariableDefinition struct {
	Location
	Typed

	Global bool

	Name  DefinedVariable
	Type  xyz.Maybe[Type]
	Value xyz.Maybe[Expression]
}

type ConstantDefinition struct {
	Location
	Typed

	Global bool

	Name  DefinedConstant
	Value Expression
}

type TypeDefinition struct {
	Location
	Typed

	Global bool

	Name           DefinedType
	TypeParameters xyz.Maybe[FieldList]
	Type           Type

	Package string
}
