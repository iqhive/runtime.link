package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"strconv"
	"strings"

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

	Binary           xyz.Case[Expression, ExpressionBinary]
	Identifier       xyz.Case[Expression, Identifier]
	Call             xyz.Case[Expression, ExpressionCall]
	Index            xyz.Case[Expression, ExpressionIndex]
	Indicies         xyz.Case[Expression, ExpressionIndicies]
	KeyValue         xyz.Case[Expression, ExpressionKeyValue]
	Parenthesized    xyz.Case[Expression, Parenthesized]
	Selector         xyz.Case[Expression, Selection]
	Slice            xyz.Case[Expression, ExpressionSlice]
	Star             xyz.Case[Expression, Star]
	TypeAssertion    xyz.Case[Expression, ExpressionTypeAssertion]
	Unary            xyz.Case[Expression, ExpressionUnary]
	Expansion        xyz.Case[Expression, ExpressionExpansion]
	LiteralBasic     xyz.Case[Expression, LiteralBasic]
	CompositeLiteral xyz.Case[Expression, CompositeLiteral]
	LiteralFunction  xyz.Case[Expression, LiteralFunction]
	Type             xyz.Case[Expression, Type]

	BuiltinFunction xyz.Case[Expression, Identifier]
}]

func (e Expression) TypeAndValue() types.TypeAndValue {
	value, _ := e.Get()
	return value.TypeAndValue()
}

func (e Expression) compile(w io.Writer) error {
	value, _ := e.Get()
	return value.compile(w)
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
		return Expressions.Indicies.New(pkg.loadExpressionIndicies(expr))
	case *ast.KeyValueExpr:
		return Expressions.KeyValue.New(pkg.loadExpressionKeyValue(expr))
	case *ast.ParenExpr:
		return Expressions.Parenthesized.New(pkg.loadParenthesized(expr))
	case *ast.SelectorExpr:
		return Expressions.Selector.New(pkg.loadSelection(expr))
	case *ast.SliceExpr:
		return Expressions.Slice.New(pkg.loadExpressionSlice(expr))
	case *ast.StarExpr:
		return Expressions.Star.New(pkg.loadStar(expr))
	case *ast.TypeAssertExpr:
		return Expressions.TypeAssertion.New(pkg.loadExpressionTypeAssertion(expr))
	case *ast.UnaryExpr:
		return Expressions.Unary.New(pkg.loadExpressionUnary(expr))
	case *ast.Ellipsis:
		return Expressions.Expansion.New(pkg.loadExpressionExpansion(expr))
	case *ast.CompositeLit:
		return Expressions.CompositeLiteral.New(pkg.loadCompositeLiteral(expr))
	case *ast.FuncLit:
		return Expressions.LiteralFunction.New(pkg.loadLiteralFunction(expr))
	case *ast.BasicLit:
		return Expressions.LiteralBasic.New(pkg.loadBasicLiteral(expr))
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

type ExpressionIndicies struct {
	typed

	X        Expression
	Opening  Location
	Indicies []Expression
	Closing  Location
}

func (pkg *Package) loadExpressionIndicies(in *ast.IndexListExpr) ExpressionIndicies {
	var out ExpressionIndicies
	out.typed = typed{pkg.Types[in]}
	out.X = pkg.loadExpression(in.X)
	out.Opening = Location(in.Lbrack)
	for _, index := range in.Indices {
		out.Indicies = append(out.Indicies, pkg.loadExpression(index))
	}
	out.Closing = Location(in.Rbrack)
	return out
}

type ExpressionKeyValue struct {
	typed

	Key   Expression
	Colon Location
	Value Expression
}

func (pkg *Package) loadExpressionKeyValue(in *ast.KeyValueExpr) ExpressionKeyValue {
	return ExpressionKeyValue{
		typed: typed{pkg.Types[in]},
		Key:   pkg.loadExpression(in.Key),
		Colon: Location(in.Colon),
		Value: pkg.loadExpression(in.Value),
	}
}

type ExpressionSlice struct {
	typed

	X        Expression
	Opening  Location
	From     xyz.Maybe[Expression]
	High     xyz.Maybe[Expression]
	Capacity xyz.Maybe[Expression]
	Closing  Location
}

func (pkg *Package) loadExpressionSlice(in *ast.SliceExpr) ExpressionSlice {
	var out ExpressionSlice
	out.typed = typed{pkg.Types[in]}
	out.X = pkg.loadExpression(in.X)
	out.Opening = Location(in.Lbrack)
	if in.Low != nil {
		out.From = xyz.New(pkg.loadExpression(in.Low))
	}
	if in.High != nil {
		out.High = xyz.New(pkg.loadExpression(in.High))
	}
	if in.Max != nil {
		out.Capacity = xyz.New(pkg.loadExpression(in.Max))
	}
	out.Closing = Location(in.Rbrack)
	return out
}

type ExpressionTypeAssertion struct {
	typed

	X       Expression
	Opening Location
	Type    Type
	Closing Location
}

func (pkg *Package) loadExpressionTypeAssertion(in *ast.TypeAssertExpr) ExpressionTypeAssertion {
	return ExpressionTypeAssertion{
		typed:   typed{pkg.Types[in]},
		X:       pkg.loadExpression(in.X),
		Opening: Location(in.Lparen),
		Type:    pkg.loadType(in.Type),
		Closing: Location(in.Rparen),
	}
}

type ExpressionUnary struct {
	typed

	Operation WithLocation[token.Token]
	X         Expression
}

func (pkg *Package) loadExpressionUnary(in *ast.UnaryExpr) ExpressionUnary {
	return ExpressionUnary{
		typed:     typed{pkg.Types[in]},
		Operation: WithLocation[token.Token]{Value: in.Op, SourceLocation: Location(in.OpPos)},
		X:         pkg.loadExpression(in.X),
	}
}

type ExpressionExpansion struct {
	typed

	Expression WithLocation[Expression]
}

func (pkg *Package) loadExpressionExpansion(in *ast.Ellipsis) ExpressionExpansion {
	return ExpressionExpansion{
		typed: typed{pkg.Types[in]},
		Expression: WithLocation[Expression]{
			Value:          pkg.loadExpression(in.Elt),
			SourceLocation: Location(in.Ellipsis),
		},
	}
}

type CompositeLiteral struct {
	typed

	Type       Type
	OpenBrace  Location
	Elements   []Expression
	CloseBrace Location
	Incomplete bool
}

func (pkg *Package) loadCompositeLiteral(in *ast.CompositeLit) CompositeLiteral {
	var out CompositeLiteral
	out.typed = typed{pkg.Types[in]}
	out.Type = pkg.loadType(in.Type)
	out.OpenBrace = Location(in.Lbrace)
	for _, expr := range in.Elts {
		out.Elements = append(out.Elements, pkg.loadExpression(expr))
	}
	out.CloseBrace = Location(in.Rbrace)
	out.Incomplete = in.Incomplete
	return out
}

type LiteralFunction struct {
	typed

	Type TypeFunction
	Body StatementBlock
}

func (pkg *Package) loadLiteralFunction(in *ast.FuncLit) LiteralFunction {
	var out LiteralFunction
	out.typed = typed{pkg.Types[in]}
	out.Type = pkg.loadTypeFunction(in.Type)
	out.Body = pkg.loadStatementBlock(in.Body)
	return out
}

type LiteralBasic struct {
	typed

	WithLocation[string]
	Kind token.Token
}

func (pkg *Package) loadBasicLiteral(in *ast.BasicLit) LiteralBasic {
	return LiteralBasic{
		typed: typed{pkg.Types[in]},
		WithLocation: WithLocation[string]{
			Value:          in.Value,
			SourceLocation: Location(in.ValuePos),
		},
		Kind: in.Kind,
	}
}

func (lit LiteralBasic) compile(w io.Writer) error {
	if lit.Kind == token.INT && len(lit.Value) > 1 {
		if lit.Value[0] == '0' && ((lit.Value[1] > '0' && lit.Value[1] < '9') || lit.Value[1] == '_') {
			// Zig does not support leading zeroes in integer
			// literals.
			_, err := w.Write([]byte("0o" + strings.TrimPrefix(lit.Value[1:], "_")))
			return err
		}
	}
	if (lit.Kind == token.IMAG || lit.Kind == token.FLOAT) && len(lit.Value) > 1 {
		lit.Value = strings.TrimSuffix(lit.Value, "i")
		if lit.Value == "0" {
			lit.Value = "0.0"
		}
		// Zig does not support leading zeros, decimal points or trailing
		// decimal points in floating point literals.
		if lit.Value[1] != 'x' && lit.Value[1] != 'o' {
			lit.Value = strings.TrimLeft(lit.Value, "0")
		}
		if lit.Value == "." {
			lit.Value = "0.0"
		}
		if lit.Value[0] == '.' {
			lit.Value = "0" + lit.Value
		}
		if lit.Value[len(lit.Value)-1] == '.' {
			lit.Value = lit.Value + "0"
		}
	}
	if lit.Kind == token.IMAG {
		fmt.Fprintf(w, "std.math.Complex(f64).init(0,%s)", lit.Value)
		return nil
	}
	if lit.Kind == token.CHAR {
		// we just convert runes into integer values.
		value, _, _, err := strconv.UnquoteChar(lit.Value[1:], '\'')
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%d", value)
		return nil
	}
	if lit.Kind == token.STRING {
		// normalize string literals, as zig has a different format for
		// unicode escape sequences.
		val, err := strconv.Unquote(lit.Value)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%q", val)
		return nil
	}
	_, err := w.Write([]byte(strings.ReplaceAll(lit.Value, "_", "")))
	return err
}

type Identifier struct {
	typed

	Name WithLocation[string]
}

func (pkg *Package) loadIdentifier(in *ast.Ident) Identifier {
	return Identifier{
		typed: typed{tv: pkg.Types[in]},
		Name: WithLocation[string]{
			Value:          in.Name,
			SourceLocation: Location(in.NamePos),
		},
	}
}

func (id Identifier) compile(w io.Writer) error {
	_, err := w.Write([]byte(id.Name.Value))
	return err
}
