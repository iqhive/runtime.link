package escape

import (
	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func (escape graph) RoutesForExpressionIndex(val source.ExpressionIndex) source.ExpressionIndex {
	val.X = escape.RoutesForExpression(val.X)
	val.Index = escape.RoutesForExpression(val.Index)
	escape.Together(val, val.X)
	return val
}

func (escape graph) RoutesForExpressionIndices(val source.ExpressionIndices) source.ExpressionIndices {
	val.X = escape.RoutesForExpression(val.X)
	for i := range val.Indicies {
		val.Indicies[i] = escape.RoutesForExpression(val.Indicies[i])
	}
	escape.Together(val, val.X)
	return val
}

func (escape graph) RoutesForExpressionSelector(val source.Selection) source.Selection {
	val.X = escape.RoutesForExpression(val.X)
	val.Selection = escape.RoutesForExpression(val.Selection)
	escape.Together(val, val.X)
	escape.Together(val, val.Selection)
	return val
}

func (escape graph) RoutesForExpressionSlice(val source.ExpressionSlice) source.ExpressionSlice {
	val.X = escape.RoutesForExpression(val.X)
	if x, ok := val.From.Get(); ok {
		val.From = xyz.New(escape.RoutesForExpression(x))
	}
	if x, ok := val.High.Get(); ok {
		val.High = xyz.New(escape.RoutesForExpression(x))
	}
	if x, ok := val.Capacity.Get(); ok {
		val.Capacity = xyz.New(escape.RoutesForExpression(x))
	}
	escape.Together(val, val.X)
	return val
}

func (escape graph) RoutesForExpressionStar(val source.Star) source.Star {
	val.WithLocation.Value = escape.RoutesForExpression(val.WithLocation.Value)
	escape.Together(val, val.WithLocation.Value)
	return val
}

func (escape graph) RoutesForExpressionBinary(val source.ExpressionBinary) source.ExpressionBinary {
	val.X = escape.RoutesForExpression(val.X)
	val.Y = escape.RoutesForExpression(val.Y)
	escape.Together(val, val.X)
	escape.Together(val, val.Y)
	return val
}

func (escape graph) RoutesForExpressionKeyValue(val source.ExpressionKeyValue) source.ExpressionKeyValue {
	val.Key = escape.RoutesForExpression(val.Key)
	val.Value = escape.RoutesForExpression(val.Value)
	escape.Together(val, val.Value)
	return val
}

func (escape graph) RoutesForExpressionParenthesized(val source.Parenthesized) source.Parenthesized {
	val.X = escape.RoutesForExpression(val.X)
	escape.Together(val, val.X)
	return val
}

func (escape graph) RoutesForExpressionUnary(val source.ExpressionUnary) source.ExpressionUnary {
	val.X = escape.RoutesForExpression(val.X)
	escape.Together(val, val.X)
	return val
}

func (escape graph) RoutesForExpressionExpansion(val source.ExpressionExpansion) source.ExpressionExpansion {
	if x, ok := val.Expression.Get(); ok {
		val.Expression = xyz.New(escape.RoutesForExpression(x))
		escape.Together(val, x)
	}
	return val
}

func (escape graph) RoutesForExpressionComposite(val source.DataComposite) source.DataComposite {
	for i := range val.Elements {
		escape.Together(val, val.Elements[i])
		val.Elements[i] = escape.RoutesForExpression(val.Elements[i])
	}
	return val
}

func (escape graph) RoutesForExpressionFunction(val source.ExpressionFunction) source.ExpressionFunction {
	val.Body = escape.RoutesForStatementBlock(val.Body)
	return val
}

func (escape graph) RoutesForExpressionBuiltinFunction(expr source.BuiltinFunction) source.BuiltinFunction {
	return expr
}

func (escape graph) RoutesForStatementRange(val source.StatementRange) source.StatementRange {
	val.X = escape.RoutesForExpression(val.X)
	val.Body = escape.RoutesForStatementBlock(val.Body)
	return val
}

func (escape graph) RoutesForExpressionTypeAssertion(val source.ExpressionTypeAssertion) source.ExpressionTypeAssertion {
	val.X = escape.RoutesForExpression(val.X)
	escape.Together(val, val.X)
	return val
}
