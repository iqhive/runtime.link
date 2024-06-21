package source

import (
	"go/ast"
	"go/types"
	"io"

	"runtime.link/xyz"
)

type TypedNode interface {
	Node
	TypeAndValue() types.TypeAndValue
}

type typed struct {
	tv types.TypeAndValue
}

func (n typed) TypeAndValue() types.TypeAndValue {
	return types.TypeAndValue(n.tv)
}

type Expression xyz.Switch[TypedNode, struct {
	Bad xyz.Case[Expression, Bad]

	Binary        xyz.Case[Expression, ExpressionBinary]
	Identifier    xyz.Case[Expression, Identifier]
	Call          xyz.Case[Expression, ExpressionCall]
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

func (e Expression) compile(w io.Writer, tabs int) error {
	value, _ := e.Get()
	return value.compile(w, tabs)
}

var Expressions = xyz.AccessorFor(Expression.Values)

func (pkg *Package) loadExpression(node ast.Expr) Expression {
	switch expr := node.(type) {
	case *ast.BadExpr:
		return Expressions.Bad.New(pkg.loadBad(expr, expr.From, expr.To))
	case *ast.BinaryExpr:
		return Expressions.Binary.New(pkg.loadExpressionBinary(expr))
	case *ast.CallExpr:
		return Expressions.Call.New(pkg.loadExpressionCall(expr))
	case *ast.IndexExpr:
		return Expressions.Index.New(pkg.loadExpressionIndex(expr))
	case *ast.IndexListExpr:
		return Expressions.Indices.New(pkg.loadExpressionIndices(expr))
	case *ast.KeyValueExpr:
		return Expressions.KeyValue.New(pkg.loadExpressionKeyValue(expr))
	case *ast.ParenExpr:
		return Expressions.Parenthesized.New(pkg.loadParenthesized(expr))
	case *ast.SelectorExpr:
		return Expressions.Selector.New(pkg.loadSelection(expr))
	case *ast.SliceExpr:
		return Expressions.Slice.New(pkg.loadExpressionSlice(expr))
	case *ast.StarExpr:
		if _, ok := pkg.TypeOf(expr).(*types.Pointer); ok {
			return Expressions.Type.New(Types.Pointer.New(pkg.loadTypePointer(expr)))
		}
		return Expressions.Star.New(pkg.loadStar(expr))
	case *ast.TypeAssertExpr:
		return Expressions.TypeAssertion.New(pkg.loadExpressionTypeAssertion(expr))
	case *ast.UnaryExpr:
		return Expressions.Unary.New(pkg.loadExpressionUnary(expr))
	case *ast.Ellipsis:
		return Expressions.Expansion.New(pkg.loadExpressionExpansion(expr))
	case *ast.CompositeLit:
		return Expressions.Composite.New(pkg.loadDataComposite(expr))
	case *ast.FuncLit:
		return Expressions.Function.New(pkg.loadExpressionFunction(expr))
	case *ast.BasicLit:
		return Expressions.Constant.New(pkg.loadConstant(expr))
	case *ast.Ident:
		switch pkg.ObjectOf(expr).(type) {
		case *types.Builtin:
			return Expressions.BuiltinFunction.New(pkg.loadIdentifier(expr))
		default:
			return Expressions.Identifier.New(pkg.loadIdentifier(expr))
		}
	default:
		return Expressions.Type.New(pkg.loadType(node))
	}
}
