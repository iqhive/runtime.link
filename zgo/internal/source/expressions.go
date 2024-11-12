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
	Identifier    xyz.Case[Expression, Identifier]
	Call          xyz.Case[Expression, ExpressionCall]
	Receive       xyz.Case[Expression, ExpressionReceive]
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
	Constant      xyz.Case[Expression, Constant]
	Composite     xyz.Case[Expression, DataComposite]
	Function      xyz.Case[Expression, ExpressionFunction]
	Type          xyz.Case[Expression, Type]

	BuiltinFunction xyz.Case[Expression, Identifier]
}]

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

type ExpressionCall struct {
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

type ExpressionReceive struct {
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
