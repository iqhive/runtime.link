package escape

import (
	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func (escape graph) RoutesForDefinition(def *source.Definition) {
	switch xyz.ValueOf(*def) {
	case source.Definitions.Function:
		*def = source.Definitions.Function.New(escape.RoutesForFunction(source.Definitions.Function.Get(*def)))
	case source.Definitions.Variable:
		val := source.Definitions.Variable.Get(*def)
		val.Name.Escapes = escape.InformationForDefinedVariable(val.Name, def)
		*def = source.Definitions.Variable.New(val)
	case source.Definitions.Constant, source.Definitions.Invalid:
		return
	}
}

func (escape graph) RoutesForStatementDefinitions(definitions source.StatementDefinitions) source.StatementDefinitions {
	for i := range definitions {
		escape.RoutesForDefinition(&definitions[i])
	}
	return definitions
}

func (escape graph) RoutesForFunction(def source.FunctionDefinition) source.FunctionDefinition {
	if body, ok := def.Body.Get(); ok {
		def.Body = xyz.New(escape.RoutesForStatementBlock(body))
	}
	return def
}

func (escape graph) RoutesForExpression(expr source.Expression) source.Expression {
	switch xyz.ValueOf(expr) {
	case source.Expressions.Bad, source.Expressions.Constant, source.Expressions.Type, source.Expressions.Nil,
		source.Expressions.ImportedPackage, source.Expressions.DefinedType, source.Expressions.DefinedFunction,
		source.Expressions.DefinedConstant:
		return expr
	case source.Expressions.DefinedVariable:
		val := source.Expressions.DefinedVariable.Get(expr)
		val.Escapes = escape.InformationForDefinedVariable(val, expr)
		return source.Expressions.DefinedVariable.New(val)
	case source.Expressions.Binary:
		return source.Expressions.Binary.New(escape.RoutesForExpressionBinary(source.Expressions.Binary.Get(expr)))
	case source.Expressions.Index:
		return source.Expressions.Index.New(escape.RoutesForExpressionIndex(source.Expressions.Index.Get(expr)))
	case source.Expressions.Indices:
		return source.Expressions.Indices.New(escape.RoutesForExpressionIndices(source.Expressions.Indices.Get(expr)))
	case source.Expressions.KeyValue:
		return source.Expressions.KeyValue.New(escape.RoutesForExpressionKeyValue(source.Expressions.KeyValue.Get(expr)))
	case source.Expressions.Parenthesized:
		return source.Expressions.Parenthesized.New(escape.RoutesForExpressionParenthesized(source.Expressions.Parenthesized.Get(expr)))
	case source.Expressions.Selector:
		return source.Expressions.Selector.New(escape.RoutesForExpressionSelector(source.Expressions.Selector.Get(expr)))
	case source.Expressions.Slice:
		return source.Expressions.Slice.New(escape.RoutesForExpressionSlice(source.Expressions.Slice.Get(expr)))
	case source.Expressions.Star:
		return source.Expressions.Star.New(escape.RoutesForExpressionStar(source.Expressions.Star.Get(expr)))
	case source.Expressions.TypeAssertion:
		return source.Expressions.TypeAssertion.New(escape.RoutesForExpressionTypeAssertion(source.Expressions.TypeAssertion.Get(expr)))
	case source.Expressions.Unary:
		return source.Expressions.Unary.New(escape.RoutesForExpressionUnary(source.Expressions.Unary.Get(expr)))
	case source.Expressions.Expansion:
		return source.Expressions.Expansion.New(escape.RoutesForExpressionExpansion(source.Expressions.Expansion.Get(expr)))
	case source.Expressions.Composite:
		return source.Expressions.Composite.New(escape.RoutesForExpressionComposite(source.Expressions.Composite.Get(expr)))
	case source.Expressions.Function:
		return source.Expressions.Function.New(escape.RoutesForExpressionFunction(source.Expressions.Function.Get(expr)))
	case source.Expressions.BuiltinFunction:
		return source.Expressions.BuiltinFunction.New(escape.RoutesForExpressionBuiltinFunction(source.Expressions.BuiltinFunction.Get(expr)))
	case source.Expressions.AwaitChannel:
		return source.Expressions.AwaitChannel.New(escape.RoutesForExpressionAwaitChannel(source.Expressions.AwaitChannel.Get(expr)))
	case source.Expressions.FunctionCall:
		return source.Expressions.FunctionCall.New(escape.RoutesForFunctionCall(source.Expressions.FunctionCall.Get(expr)))
	default:
		return expr
	}
}

func (escape graph) RoutesForStatementBlock(block source.StatementBlock) source.StatementBlock {
	for i := range block.Statements {
		block.Statements[i] = escape.RoutesForStatement(block.Statements[i])
	}
	return block
}

func (escape graph) RoutesForStatement(stmt source.Statement) source.Statement {
	switch xyz.ValueOf(stmt) {
	case source.Statements.Bad, source.Statements.Goto, source.Statements.Empty, source.Statements.Increment, source.Statements.Decrement:
		return stmt
	case source.Statements.Assignment:
		return source.Statements.Assignment.New(escape.RoutesForStatementAssignment(source.Statements.Assignment.Get(stmt)))
	case source.Statements.Block:
		return source.Statements.Block.New(escape.RoutesForStatementBlock(source.Statements.Block.Get(stmt)))
	case source.Statements.Definitions:
		return source.Statements.Definitions.New(escape.RoutesForStatementDefinitions(source.Statements.Definitions.Get(stmt)))
	case source.Statements.Defer:
		return source.Statements.Defer.New(escape.RoutesForStatmentDefer(source.Statements.Defer.Get(stmt)))
	case source.Statements.Expression:
		return source.Statements.Expression.New(escape.RoutesForExpression(source.Statements.Expression.Get(stmt)))
	case source.Statements.Go:
		return source.Statements.Go.New(escape.RoutesForStatementGo(source.Statements.Go.Get(stmt)))
	case source.Statements.If:
		return source.Statements.If.New(escape.RoutesForStatementIf(source.Statements.If.Get(stmt)))
	case source.Statements.For:
		return source.Statements.For.New(escape.RoutesForStatementFor(source.Statements.For.Get(stmt)))
	case source.Statements.Label:
		return source.Statements.Label.New(escape.RoutesForStatementLabel(source.Statements.Label.Get(stmt)))
	case source.Statements.Range:
		return source.Statements.Range.New(escape.RoutesForStatementRange(source.Statements.Range.Get(stmt)))
	case source.Statements.Return:
		return source.Statements.Return.New(escape.RoutesForStatementReturn(source.Statements.Return.Get(stmt)))
	case source.Statements.Select:
		return source.Statements.Select.New(escape.RoutesForStatementSelect(source.Statements.Select.Get(stmt)))
	case source.Statements.Send:
		return source.Statements.Send.New(escape.RoutesForStatementSend(source.Statements.Send.Get(stmt)))
	case source.Statements.SwitchType:
		return source.Statements.SwitchType.New(escape.RoutesForStatementSwitchType(source.Statements.SwitchType.Get(stmt)))
	case source.Statements.Switch:
		return source.Statements.Switch.New(escape.RoutesForStatementSwitch(source.Statements.Switch.Get(stmt)))
	case source.Statements.Continue:
		return stmt
	case source.Statements.Break:
		return stmt
	case source.Statements.Fallthrough:
		return stmt
	default:
		return stmt
	}
}

func (escape graph) RoutesForStatementIf(val source.StatementIf) source.StatementIf {
	if s, ok := val.Init.Get(); ok {
		val.Init = xyz.New(escape.RoutesForStatement(s))
	}
	val.Condition = escape.RoutesForExpression(val.Condition)
	val.Body = escape.RoutesForStatementBlock(val.Body)
	if e, ok := val.Else.Get(); ok {
		val.Else = xyz.New(escape.RoutesForStatement(e))
	}
	return val
}

func (escape graph) RoutesForStatementFor(val source.StatementFor) source.StatementFor {
	if s, ok := val.Init.Get(); ok {
		val.Init = xyz.New(escape.RoutesForStatement(s))
	}
	if e, ok := val.Condition.Get(); ok {
		val.Condition = xyz.New(escape.RoutesForExpression(e))
	}
	if s, ok := val.Statement.Get(); ok {
		val.Statement = xyz.New(escape.RoutesForStatement(s))
	}
	val.Body = escape.RoutesForStatementBlock(val.Body)
	return val
}

func (escape graph) RoutesForStatementLabel(val source.StatementLabel) source.StatementLabel {
	val.Statement = escape.RoutesForStatement(val.Statement)
	return val
}

func (escape graph) RoutesForStatementSelect(val source.StatementSelect) source.StatementSelect {
	for i := range val.Clauses {
		cl := &val.Clauses[i]
		if s, ok := cl.Statement.Get(); ok {
			cl.Statement = xyz.New(escape.RoutesForStatement(s))
		}
		for j := range cl.Body {
			cl.Body[j] = escape.RoutesForStatement(cl.Body[j])
		}
	}
	return val
}

func (escape graph) RoutesForStatementSwitchType(val source.StatementSwitchType) source.StatementSwitchType {
	if s, ok := val.Init.Get(); ok {
		val.Init = xyz.New(escape.RoutesForStatement(s))
	}
	val.Assign = escape.RoutesForStatement(val.Assign)
	for i := range val.Claused {
		cc := &val.Claused[i]
		for j := range cc.Body {
			cc.Body[j] = escape.RoutesForStatement(cc.Body[j])
		}
	}
	return val
}

func (escape graph) RoutesForStatementSwitch(val source.StatementSwitch) source.StatementSwitch {
	if s, ok := val.Init.Get(); ok {
		val.Init = xyz.New(escape.RoutesForStatement(s))
	}
	if e, ok := val.Value.Get(); ok {
		val.Value = xyz.New(escape.RoutesForExpression(e))
	}
	for i := range val.Clauses {
		cc := &val.Clauses[i]
		for j := range cc.Expressions {
			cc.Expressions[j] = escape.RoutesForExpression(cc.Expressions[j])
		}
		for j := range cc.Body {
			cc.Body[j] = escape.RoutesForStatement(cc.Body[j])
		}
	}
	return val
}
