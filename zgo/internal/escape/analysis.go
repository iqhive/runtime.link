package escape

import (
	"go/ast"
	"go/types"
	"slices"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

// Analysis performs escape analysis on the given package, returning the package
// with escape information attached.
func Analysis(pkg source.Package) source.Package {
	var escape = make(graph)
	for i := range pkg.Files {
		escape.RoutesForFile(&pkg.Files[i])
	}
	return pkg
}

type graph map[graphKey]plan

type graphKey xyz.Tagged[any, struct {
	Node     xyz.Case[graphKey, ast.Node]
	Variable xyz.Case[graphKey, source.Unique]
}]

var graphKeys = xyz.AccessorFor(graphKey.Values)

// Route marks the given expression as an escape route.
func (escape graph) Route(expr source.Node, via plan) {
	key := graphKeys.Node.New(source.LocationOf(expr).Node)
	escape[key] = escape[key].route(via)
}

// Together links two expressions, indicating that if one escapes, the
// other does too.
func (escape graph) Together(a, b source.Node) {
	keyA := graphKeys.Node.New(source.LocationOf(a).Node)
	keyB := graphKeys.Node.New(source.LocationOf(b).Node)
	if keyA == keyB {
		return // no need to link the same node
	}
	escape[keyA] = escape[keyA].route(plan{accomplices: []graphKey{keyB}})
	escape[keyB] = escape[keyB].route(plan{accomplices: []graphKey{keyA}})
}

// Make returns the escape information functions for the given node.
func (escape graph) InformationForDefinedVariable(val source.DefinedVariable, with source.Node) source.EscapeInformation {
	keyA := graphKeys.Variable.New(val.Unique)
	keyB := graphKeys.Node.New(source.LocationOf(with).Node)
	escape[keyA] = escape[keyA].route(plan{accomplices: []graphKey{keyB}})
	escape[keyB] = escape[keyB].route(plan{accomplices: []graphKey{keyA}})
	return source.EscapeInformation{
		Block: func() source.EscapeFeasibility {
			return source.EscapeFeasibility{
				Possible: escape.get(keyA, map[graphKey]struct{}{}).block,
			}
		},
		Function: func() source.EscapeFeasibility {
			return source.EscapeFeasibility{
				Possible: escape.get(keyA, map[graphKey]struct{}{}).function,
			}
		},
		Goroutine: func() source.EscapeFeasibility {
			return source.EscapeFeasibility{
				Possible: escape.get(keyA, map[graphKey]struct{}{}).goroutine,
			}
		},
		Containment: func() source.EscapeFeasibility {
			return source.EscapeFeasibility{
				Possible: escape.get(keyA, map[graphKey]struct{}{}).containment,
			}
		},
	}
}

func (escape graph) get(key graphKey, seen map[graphKey]struct{}) plan {
	if _, ok := seen[key]; ok {
		return plan{}
	}
	seen[key] = struct{}{}
	p, ok := escape[key]
	if !ok {
		return plan{}
	}
	for _, accomplice := range p.accomplices {
		p = p.route(escape.get(accomplice, seen))
	}
	escape[key] = p // cache the result
	return p
}

type plan struct {
	block, function, goroutine, containment bool

	accomplices []graphKey
}

func (escape plan) route(via plan) plan {
	if via.block {
		escape.block = true
	}
	if via.function {
		escape.function = true
	}
	if via.goroutine {
		escape.goroutine = true
	}
	if via.containment {
		escape.containment = true
	}
	for _, buddy := range via.accomplices {
		if !slices.Contains(escape.accomplices, buddy) {
			escape.accomplices = append(escape.accomplices, buddy)
		}
	}
	return escape
}

// RoutesForFunctionCall TODO/FIXME: assess arguments.
func (escape graph) RoutesForFunctionCall(call source.FunctionCall) source.FunctionCall {
	call.Function = escape.RoutesForExpression(call.Function)
	for i := range call.Arguments {
		escape.Route(call.Arguments[i], plan{
			block:       true,
			function:    true,
			goroutine:   true,
			containment: true,
		})
		call.Arguments[i] = escape.RoutesForExpression(call.Arguments[i])
	}
	return call
}

// TODO/FIXME: escapes block
func (escape graph) RoutesForStatmentDefer(val source.StatementDefer) source.StatementDefer {
	val.Call = escape.RoutesForFunctionCall(val.Call)
	return val
}

// RoutesForStatementAssignment checks for escape routes in an assignment statement:
//
//   - if being assigned to an outer scope, marks this as a block-escape route.
//   - if being assigned to a function argument/result, marks this as a function-escape route.
//   - if being assigned to a global, marks this as a route that escapes containment.
//
// TODO/FIXME handle assignment across function-boundaries.
func (escape graph) RoutesForStatementAssignment(statement source.StatementAssignment) source.StatementAssignment {
	for i := range statement.Variables {
		statement.Variables[i] = escape.RoutesForExpression(statement.Variables[i])
	}
	for i := range statement.Values {
		escape.Route(statement.Values[i], plan{
			block:       isOuterScopeTo(statement, statement.Variables[i]),
			containment: isGlobal(statement.Variables[i]),
			function:    isFunctionDefined(statement.Variables[i]),
		})
		escape.Together(statement.Variables[i], statement.Values[i])
		statement.Values[i] = escape.RoutesForExpression(statement.Values[i])
	}
	return statement
}

func (escape graph) RoutesForStatementGo(val source.StatementGo) source.StatementGo {
	//goNode := source.LocationOf(val).Node
	//_ = escape.Make(goNode, plan{function: true, goroutine: true})
	val.Call = escape.RoutesForFunctionCall(val.Call)
	return val
}

// RoutesForStatementReturn marks all pass-by-reference return values as escaping.
func (escape graph) RoutesForStatementReturn(val source.StatementReturn) source.StatementReturn {
	for i := range val.Results {
		switch underlying := val.Results[i].TypeAndValue().Type.Underlying().(type) {
		case *types.Basic:
			if underlying.Kind() == types.String {
				escape.Route(val.Results[i], plan{function: true})
			}
		case *types.Chan, *types.Signature, *types.Interface, *types.Map, *types.Pointer, *types.Slice:
			escape.Route(val.Results[i], plan{function: true})
		}
		val.Results[i] = escape.RoutesForExpression(val.Results[i])
	}
	return val
}

func (escape graph) RoutesForExpressionAwaitChannel(val source.AwaitChannel) source.AwaitChannel {
	val.Chan = escape.RoutesForExpression(val.Chan)
	return val
}

func (escape graph) RoutesForStatementSend(val source.StatementSend) source.StatementSend {
	val.X = escape.RoutesForExpression(val.X)
	val.Value = escape.RoutesForExpression(val.Value)
	escape.Together(val.X, val.Value)
	return val
}

func isGlobal(expr source.Expression) bool {
	switch xyz.ValueOf(expr) {
	case source.Expressions.DefinedVariable:
		variable := source.Expressions.DefinedVariable.Get(expr)
		return variable.Package
	case source.Expressions.Selector:
		selector := source.Expressions.Selector.Get(expr)
		if xyz.ValueOf(selector.Selection) == source.Expressions.DefinedVariable {
			variable := source.Expressions.DefinedVariable.Get(selector.Selection)
			return variable.Package
		}
	}
	return false
}
