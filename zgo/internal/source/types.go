package source

import (
	"go/ast"
	"go/types"

	"runtime.link/xyz"
)

type Type xyz.Tagged[TypedNode, struct {
	Bad xyz.Case[Type, Bad]

	Unknown xyz.Case[Type, TypeUnknown]

	Identifier    xyz.Case[Type, Identifier]
	Parenthesized xyz.Case[Type, Parenthesized]
	Selection     xyz.Case[Type, Selection]
	TypeArray     xyz.Case[Type, TypeArray]
	TypeChannel   xyz.Case[Type, TypeChannel]
	TypeFunction  xyz.Case[Type, TypeFunction]
	TypeInterface xyz.Case[Type, TypeInterface]
	TypeMap       xyz.Case[Type, TypeMap]
	TypeStruct    xyz.Case[Type, TypeStruct]
	TypeVariadic  xyz.Case[Type, TypeVariadic]
	Pointer       xyz.Case[Type, TypePointer]
}]

var Types = xyz.AccessorFor(Type.Values)

func (e Type) sources() Location {
	value, _ := e.Get()
	return value.sources()
}

func (e Type) TypeAndValue() types.TypeAndValue {
	value, _ := e.Get()
	return value.TypeAndValue()
}

type TypeUnknown struct {
	Typed
	Location
}

type TypeArray struct {
	Typed

	Location

	OpenBracket Location
	Length      xyz.Maybe[Expression]
	ElementType Type
}

type TypeChannel struct {
	Typed

	Location

	Begin Location
	Arrow Location
	Dir   ast.ChanDir
	Value Expression
}

type TypePointer Star

type TypeFunction struct {
	Typed

	Location

	Keyword    Location
	TypeParams xyz.Maybe[FieldList]
	Arguments  FieldList
	Results    xyz.Maybe[FieldList]
}

type TypeInterface struct {
	Typed

	Location

	Keyword    Location
	Methods    FieldList
	Incomplete bool
}

type TypeMap struct {
	Typed

	Location

	Keyword Location
	Key     Expression
	Value   Expression
}

type TypeStruct struct {
	Typed

	Location

	Keyword    Location
	Fields     FieldList
	Incomplete bool
}

type TypeVariadic struct {
	Typed

	Location

	ElementType WithLocation[Type]
}

type Typed struct {
	TV  types.TypeAndValue
	PKG string
}

func (pkg *Package) typed(node ast.Expr) Typed {
	return Typed{pkg.Types[node], pkg.Name}
}

func (n Typed) TypeAndValue() types.TypeAndValue {
	return types.TypeAndValue(n.TV)
}
