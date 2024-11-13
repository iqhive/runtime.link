package source

import (
	"go/token"
	"go/types"

	"runtime.link/xyz"
)

type TypedNode interface {
	Node
	TypeAndValue() types.TypeAndValue
}

type Expression xyz.Tagged[TypedNode, struct {
	Bad xyz.Case[Expression, Bad]

	Binary        xyz.Case[Expression, ExpressionBinary]
	Index         xyz.Case[Expression, ExpressionIndex]
	Indices       xyz.Case[Expression, ExpressionIndices]
	KeyValue      xyz.Case[Expression, ExpressionKeyValue]
	Parenthesized xyz.Case[Expression, Parenthesized]
	Selector      xyz.Case[Expression, Selection]
	Slice         xyz.Case[Expression, ExpressionSlice]
	Star          xyz.Case[Expression, Star]
	TypeAssertion xyz.Case[Expression, ExpressionTypeAssertion]
	Unary         xyz.Case[Expression, ExpressionUnary]
	Expansion     xyz.Case[Expression, ExpressionExpansion]
	Constant      xyz.Case[Expression, Literal]
	Composite     xyz.Case[Expression, DataComposite]
	Function      xyz.Case[Expression, ExpressionFunction]
	Type          xyz.Case[Expression, Type]

	Nil             xyz.Case[Expression, Nil]
	BuiltinFunction xyz.Case[Expression, BuiltinFunction]
	ImportedPackage xyz.Case[Expression, ImportedPackage]
	DefinedType     xyz.Case[Expression, DefinedType]
	DefinedFunction xyz.Case[Expression, DefinedFunction]
	DefinedVariable xyz.Case[Expression, DefinedVariable]
	DefinedConstant xyz.Case[Expression, DefinedConstant]

	AwaitChannel xyz.Case[Expression, AwaitChannel]
	FunctionCall xyz.Case[Expression, FunctionCall]
}]

type Nil Identifier

func (n Nil) sources() Location { return n.Location }

type BuiltinFunction Identifier

func (b BuiltinFunction) sources() Location { return b.Location }

type ImportedPackage Identifier

func (i ImportedPackage) sources() Location { return i.Location }

type DefinedType Identifier

func (d DefinedType) sources() Location { return d.Location }

type DefinedFunction Identifier

func (d DefinedFunction) sources() Location { return d.Location }

type DefinedVariable Identifier

func (d DefinedVariable) sources() Location { return d.Location }

type DefinedConstant Identifier

func (d DefinedConstant) sources() Location { return d.Location }

func (e Expression) sources() Location {
	value, _ := e.Get()
	return value.sources()
}

func (e Expression) TypeAndValue() types.TypeAndValue {
	value, _ := e.Get()
	return value.TypeAndValue()
}

var Expressions = xyz.AccessorFor(Expression.Values)

type ExpressionBinary struct {
	Location

	Typed

	X         Expression
	Operation WithLocation[token.Token]
	Y         Expression
}

type FunctionCall struct {
	Location

	Typed

	Go bool

	Function  Expression
	Opening   Location
	Arguments []Expression
	Ellipsis  Location
	Closing   Location
}

type ExpressionExpansion struct {
	Typed

	Location

	Expression xyz.Maybe[Expression]
}

type ExpressionFunction struct {
	Typed

	Location

	Type TypeFunction
	Body StatementBlock
}

type ExpressionIndex struct {
	Location

	Typed

	X       Expression
	Opening Location
	Index   Expression
	Closing Location
}

type ExpressionIndices struct {
	Location

	Typed

	X        Expression
	Opening  Location
	Indicies []Expression
	Closing  Location
}

type ExpressionKeyValue struct {
	Typed

	Location

	Key   Expression
	Colon Location
	Value Expression
}

type AwaitChannel struct {
	Typed

	Location

	Chan Expression
}

type ExpressionSlice struct {
	Typed

	Location

	X        Expression
	Opening  Location
	From     xyz.Maybe[Expression]
	High     xyz.Maybe[Expression]
	Capacity xyz.Maybe[Expression]
	Closing  Location
}

type ExpressionTypeAssertion struct {
	Typed

	Location

	X       Expression
	Opening Location
	Type    xyz.Maybe[Type]
	Closing Location
}

type ExpressionUnary struct {
	Typed

	Location

	Operation WithLocation[token.Token]
	X         Expression
}
