package parser

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func loadExpression(pkg *source.Package, node ast.Expr) source.Expression {
	switch expr := node.(type) {
	case *ast.BadExpr:
		return source.Expressions.Bad.New(loadBad(pkg, expr, expr.From, expr.To))
	case *ast.BinaryExpr:
		return source.Expressions.Binary.New(loadExpressionBinary(pkg, expr))
	case *ast.CallExpr:
		return source.Expressions.Call.New(loadExpressionCall(pkg, expr))
	case *ast.IndexExpr:
		return source.Expressions.Index.New(loadExpressionIndex(pkg, expr))
	case *ast.IndexListExpr:
		return source.Expressions.Indices.New(loadExpressionIndices(pkg, expr))
	case *ast.KeyValueExpr:
		return source.Expressions.KeyValue.New(loadExpressionKeyValue(pkg, expr))
	case *ast.ParenExpr:
		return source.Expressions.Parenthesized.New(loadParenthesized(pkg, expr))
	case *ast.SelectorExpr:
		return source.Expressions.Selector.New(loadSelection(pkg, expr))
	case *ast.SliceExpr:
		return source.Expressions.Slice.New(loadExpressionSlice(pkg, expr))
	case *ast.StarExpr:
		if _, ok := pkg.TypeOf(expr).(*types.Pointer); ok {
			return source.Expressions.Type.New(source.Types.Pointer.New(loadTypePointer(pkg, expr)))
		}
		return source.Expressions.Star.New(loadStar(pkg, expr))
	case *ast.TypeAssertExpr:
		return source.Expressions.TypeAssertion.New(loadExpressionTypeAssertion(pkg, expr))
	case *ast.UnaryExpr:
		if expr.Op == token.ARROW {
			return source.Expressions.Receive.New(loadExpressionReceive(pkg, expr))
		}
		return source.Expressions.Unary.New(loadExpressionUnary(pkg, expr))
	case *ast.Ellipsis:
		return source.Expressions.Expansion.New(loadExpressionExpansion(pkg, expr))
	case *ast.CompositeLit:
		return source.Expressions.Composite.New(loadDataComposite(pkg, expr))
	case *ast.FuncLit:
		return source.Expressions.Function.New(loadExpressionFunction(pkg, expr))
	case *ast.BasicLit:
		return source.Expressions.Constant.New(loadConstant(pkg, expr))
	case *ast.Ident:
		switch ident := pkg.ObjectOf(expr).(type) {
		case *types.Builtin:
			return source.Expressions.BuiltinFunction.New(source.BuiltinFunction(loadIdentifier(pkg, expr)))
		case *types.Nil:
			return source.Expressions.Nil.New(source.Nil(loadIdentifier(pkg, expr)))
		case *types.Const:
			return source.Expressions.DefinedConstant.New(source.DefinedConstant(loadIdentifier(pkg, expr)))
		case nil, *types.Var:
			return source.Expressions.DefinedVariable.New(source.DefinedVariable(loadIdentifier(pkg, expr)))
		case *types.TypeName:
			return source.Expressions.DefinedType.New(source.DefinedType(loadIdentifier(pkg, expr)))
		case *types.Func:
			return source.Expressions.DefinedFunction.New(source.DefinedFunction(loadIdentifier(pkg, expr)))
		case *types.PkgName:
			id := loadIdentifier(pkg, expr)
			id.String = ident.Name()
			return source.Expressions.ImportedPackage.New(source.ImportedPackage(id))
		default:
			panic("unsupported ident type" + reflect.TypeOf(ident).String())
		}
	default:
		return source.Expressions.Type.New(loadType(pkg, node))
	}
}

func loadBad(pkg *source.Package, node ast.Node, from, upto token.Pos) source.Bad {
	return source.Bad(locationRangeIn(pkg, from, upto))
}

func loadParenthesized(pkg *source.Package, in *ast.ParenExpr) source.Parenthesized {
	return source.Parenthesized{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		Typed:    typedIn(pkg, in),
		Opening:  locationIn(pkg, in.Lparen),
		X:        loadExpression(pkg, in.X),
		Closing:  locationIn(pkg, in.Rparen),
	}
}

func loadExpressionBinary(pkg *source.Package, in *ast.BinaryExpr) source.ExpressionBinary {
	return source.ExpressionBinary{
		Location:  locationRangeIn(pkg, in.Pos(), in.End()),
		Typed:     typedIn(pkg, in),
		X:         loadExpression(pkg, in.X),
		Operation: source.WithLocation[token.Token]{Value: in.Op, SourceLocation: locationIn(pkg, in.OpPos)},
		Y:         loadExpression(pkg, in.Y),
	}
}

func loadExpressionCall(pkg *source.Package, in *ast.CallExpr) source.ExpressionCall {
	var out source.ExpressionCall
	out.Location = locationRangeIn(pkg, in.Pos(), in.End())
	out.Typed = typedIn(pkg, in)
	out.Function = loadExpression(pkg, in.Fun)
	out.Opening = locationIn(pkg, in.Lparen)
	for _, arg := range in.Args {
		out.Arguments = append(out.Arguments, loadExpression(pkg, arg))
	}
	out.Ellipsis = locationIn(pkg, in.Ellipsis)
	out.Closing = locationIn(pkg, in.Rparen)
	return out
}

func loadConstant(pkg *source.Package, in *ast.BasicLit) source.Literal {
	return source.Literal{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		Typed:    typedIn(pkg, in),
		WithLocation: source.WithLocation[string]{
			Value:          in.Value,
			SourceLocation: locationIn(pkg, in.ValuePos),
		},
		Kind: in.Kind,
	}
}

func loadExpressionExpansion(pkg *source.Package, in *ast.Ellipsis) source.ExpressionExpansion {
	var expression xyz.Maybe[source.Expression]
	if in.Elt != nil {
		expression = xyz.New(loadExpression(pkg, in.Elt))
	}
	return source.ExpressionExpansion{
		Location:   locationRangeIn(pkg, in.Pos(), in.End()),
		Typed:      typedIn(pkg, in),
		Expression: expression,
	}
}

func loadExpressionFunction(pkg *source.Package, in *ast.FuncLit) source.ExpressionFunction {
	var out source.ExpressionFunction
	out.Location = locationRangeIn(pkg, in.Pos(), in.End())
	out.Typed = typedIn(pkg, in)
	out.Type = loadTypeFunction(pkg, in.Type)
	out.Body = loadStatementBlock(pkg, in.Body)
	return out
}

func loadExpressionIndex(pkg *source.Package, in *ast.IndexExpr) source.ExpressionIndex {
	return source.ExpressionIndex{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		Typed:    typedIn(pkg, in),
		X:        loadExpression(pkg, in.X),
		Opening:  locationIn(pkg, in.Lbrack),
		Index:    loadExpression(pkg, in.Index),
		Closing:  locationIn(pkg, in.Rbrack),
	}
}

func loadExpressionIndices(pkg *source.Package, in *ast.IndexListExpr) source.ExpressionIndices {
	var out source.ExpressionIndices
	out.Location = locationRangeIn(pkg, in.Pos(), in.End())
	out.Typed = typedIn(pkg, in)
	out.X = loadExpression(pkg, in.X)
	out.Opening = locationIn(pkg, in.Lbrack)
	for _, index := range in.Indices {
		out.Indicies = append(out.Indicies, loadExpression(pkg, index))
	}
	out.Closing = locationIn(pkg, in.Rbrack)
	return out
}

func loadExpressionKeyValue(pkg *source.Package, in *ast.KeyValueExpr) source.ExpressionKeyValue {
	return source.ExpressionKeyValue{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		Typed:    typedIn(pkg, in),
		Key:      loadExpression(pkg, in.Key),
		Colon:    locationIn(pkg, in.Colon),
		Value:    loadExpression(pkg, in.Value),
	}
}

func loadExpressionReceive(pkg *source.Package, in *ast.UnaryExpr) source.ExpressionReceive {
	return source.ExpressionReceive{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		Typed:    typedIn(pkg, in),
		Chan:     loadExpression(pkg, in.X),
	}
}

func loadExpressionSlice(pkg *source.Package, in *ast.SliceExpr) source.ExpressionSlice {
	var out source.ExpressionSlice
	out.Location = locationRangeIn(pkg, in.Pos(), in.End())
	out.Typed = typedIn(pkg, in)
	out.X = loadExpression(pkg, in.X)
	out.Opening = locationIn(pkg, in.Lbrack)
	if in.Low != nil {
		out.From = xyz.New(loadExpression(pkg, in.Low))
	}
	if in.High != nil {
		out.High = xyz.New(loadExpression(pkg, in.High))
	}
	if in.Max != nil {
		out.Capacity = xyz.New(loadExpression(pkg, in.Max))
	}
	out.Closing = locationIn(pkg, in.Rbrack)
	return out
}

func loadExpressionTypeAssertion(pkg *source.Package, in *ast.TypeAssertExpr) source.ExpressionTypeAssertion {
	var stype xyz.Maybe[source.Type]
	if in.Type != nil {
		stype = xyz.New(loadType(pkg, in.Type))
	}
	return source.ExpressionTypeAssertion{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		Typed:    typedIn(pkg, in),
		X:        loadExpression(pkg, in.X),
		Opening:  locationIn(pkg, in.Lparen),
		Type:     stype,
		Closing:  locationIn(pkg, in.Rparen),
	}
}

func loadExpressionUnary(pkg *source.Package, in *ast.UnaryExpr) source.ExpressionUnary {
	return source.ExpressionUnary{
		Location:  locationRangeIn(pkg, in.Pos(), in.End()),
		Typed:     typedIn(pkg, in),
		Operation: source.WithLocation[token.Token]{Value: in.Op, SourceLocation: locationIn(pkg, in.OpPos)},
		X:         loadExpression(pkg, in.X),
	}
}
