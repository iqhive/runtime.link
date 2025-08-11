package escape

import (
	"go/ast"
	"slices"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func Analysis(pkg source.Package) source.Package {
	var escape = make(graph)
	for i := range pkg.Files {
		escape.RouteFile(&pkg.Files[i])
	}
	return pkg
}

type graph map[ast.Node]information

func (escape graph) Make(node ast.Node, escapes bool, buddies []ast.Node) func() bool {
	escape[node] = information{
		escapes: escapes,
		buddies: buddies,
	}
	for _, buddy := range buddies {
		existing := escape[buddy]
		if !slices.Contains(existing.buddies, node) {
			existing.buddies = append(existing.buddies, node)
		}
		escape[buddy] = existing
	}
	return func() bool {
		info, ok := escape[node]
		if !ok {
			return false
		}
		if info.escapes {
			return true
		}
		var seen = make(map[ast.Node]struct{})
		for _, buddy := range info.buddies {
			if escape.view(seen, &buddy) {
				info.escapes = true
				escape[node] = info
				return true
			}
		}
		return false
	}
}

func (escape graph) view(seen map[ast.Node]struct{}, node *ast.Node) bool {
	if _, ok := seen[*node]; ok {
		return false
	}
	seen[*node] = struct{}{}
	info, ok := escape[*node]
	if !ok {
		return false
	}
	if info.escapes {
		return true
	}
	for _, buddy := range info.buddies {
		if escape.view(seen, &buddy) {
			info.escapes = true
			escape[*node] = info
			return true
		}
	}
	return false
}

type information struct {
	escapes bool
	buddies []ast.Node
}

func (escape graph) RouteFile(file *source.File) {
	for i := range file.Definitions {
		escape.RouteDefinition(&file.Definitions[i])
	}
}

func (escape graph) RouteDefinition(def *source.Definition) {
	switch xyz.ValueOf(*def) {
	case source.Definitions.Function:
		*def = source.Definitions.Function.New(escape.RouteFunction(source.Definitions.Function.Get(*def)))
	case source.Definitions.Variable:
		*def = source.Definitions.Variable.New(escape.RouteVariable(source.Definitions.Variable.Get(*def)))
	}
}

type routing struct {
	parentBuddy       ast.Node
	namedResultBuddies []ast.Node
}

func (escape graph) RouteFunction(def source.FunctionDefinition) source.FunctionDefinition {
	var buddies []ast.Node
	if def.Body.Ok {
		buddies = append(buddies, def.Body.Get().Node)
	}
	def.Type = escape.RouteTypeFunction(def.Type)

	if def.Receiver.Ok {
		recv := def.Receiver.Get()
		escape.routeField(&recv)
		if recv.Names.Ok {
			for i := range recv.Names.Get() {
				name := &recv.Names.Get()[i]
				name.Escapes = escape.Make(name.Node, false, buddies)
			}
		}
		def.Receiver = xyz.New(recv)
	}

	args := def.Type.Arguments
	for i := range args.Fields {
		f := &args.Fields[i]
		escape.routeField(f)
		if f.Names.Ok {
			for j := range f.Names.Get() {
				name := &f.Names.Get()[j]
				name.Escapes = escape.Make(name.Node, false, buddies)
			}
		}
	}

	var namedResults []ast.Node
	if def.Type.Results.Ok {
		results := def.Type.Results.Get()
		for i := range results.Fields {
			f := &results.Fields[i]
			escape.routeField(f)
			if f.Names.Ok {
				for j := range f.Names.Get() {
					name := &f.Names.Get()[j]
					name.Escapes = escape.Make(name.Node, false, buddies)
					namedResults = append(namedResults, name.Node)
				}
			}
		}
		def.Type.Results = xyz.New(results)
	}

	if def.Body.Ok {
		body := def.Body.Get()
		body = escape.routeStatementBlockCtx(body, routing{namedResultBuddies: namedResults})
		def.Body = xyz.New(body)
	}

	return def
}

func (escape graph) RouteVariable(def source.VariableDefinition) source.VariableDefinition {
	var escapes bool
	var buddies []ast.Node
	if def.Global {
		escapes = true
	}
	if value, ok := def.Value.Get(); ok {
		node, _ := value.Get()
		buddies = append(buddies, source.LocationOf(node).Node)
		def.Value = xyz.New(escape.RouteExpression(value))
	}
	def.Name.Escapes = escape.Make(def.Name.Node, escapes, buddies)
	return def
}

func (escape graph) exprNode(expr source.Expression) ast.Node {
	node, _ := expr.Get()
	return source.LocationOf(node).Node
}

func (escape graph) RouteExpression(expr source.Expression) source.Expression {
	return escape.routeExpressionWithBuddy(expr, nil)
}

func (escape graph) routeExpressionWithBuddy(expr source.Expression, parent ast.Node) source.Expression {
	switch xyz.ValueOf(expr) {
	case source.Expressions.Bad:
		return expr
	case source.Expressions.Binary:
		val := source.Expressions.Binary.Get(expr)
		self := source.LocationOf(val).Node
		val.X = xyz.New(escape.routeExpressionWithBuddy(val.X, self))
		val.Y = xyz.New(escape.routeExpressionWithBuddy(val.Y, self))
		return source.Expressions.Binary.New(val)
	case source.Expressions.Index:
		val := source.Expressions.Index.Get(expr)
		self := source.LocationOf(val).Node
		val.X = xyz.New(escape.routeExpressionWithBuddy(val.X, self))
		val.Index = xyz.New(escape.routeExpressionWithBuddy(val.Index, self))
		return source.Expressions.Index.New(val)
	case source.Expressions.Indices:
		val := source.Expressions.Indices.Get(expr)
		self := source.LocationOf(val).Node
		val.X = xyz.New(escape.routeExpressionWithBuddy(val.X, self))
		for i := range val.Indicies {
			val.Indicies[i] = escape.routeExpressionWithBuddy(val.Indicies[i], self)
		}
		return source.Expressions.Indices.New(val)
	case source.Expressions.KeyValue:
		val := source.Expressions.KeyValue.Get(expr)
		self := source.LocationOf(val).Node
		val.Key = xyz.New(escape.routeExpressionWithBuddy(val.Key, self))
		val.Value = xyz.New(escape.routeExpressionWithBuddy(val.Value, self))
		return source.Expressions.KeyValue.New(val)
	case source.Expressions.Parenthesized:
		val := source.Expressions.Parenthesized.Get(expr)
		self := source.LocationOf(val).Node
		val.X = xyz.New(escape.routeExpressionWithBuddy(val.X, self))
		return source.Expressions.Parenthesized.New(val)
	case source.Expressions.Selector:
		val := source.Expressions.Selector.Get(expr)
		self := source.LocationOf(val).Node
		val.X = xyz.New(escape.routeExpressionWithBuddy(val.X, self))
		val.Selection = xyz.New(escape.routeExpressionWithBuddy(val.Selection, self))
		return source.Expressions.Selector.New(val)
	case source.Expressions.Slice:
		val := source.Expressions.Slice.Get(expr)
		self := source.LocationOf(val).Node
		val.X = xyz.New(escape.routeExpressionWithBuddy(val.X, self))
		if x, ok := val.From.Get(); ok {
			val.From = xyz.New(escape.routeExpressionWithBuddy(x, self))
		}
		if x, ok := val.High.Get(); ok {
			val.High = xyz.New(escape.routeExpressionWithBuddy(x, self))
		}
		if x, ok := val.Capacity.Get(); ok {
			val.Capacity = xyz.New(escape.routeExpressionWithBuddy(x, self))
		}
		return source.Expressions.Slice.New(val)
	case source.Expressions.Star:
		val := source.Expressions.Star.Get(expr)
		self := source.LocationOf(val).Node
		val.WithLocation.Value = escape.routeExpressionWithBuddy(val.WithLocation.Value, self)
		return source.Expressions.Star.New(val)
	case source.Expressions.TypeAssertion:
		val := source.Expressions.TypeAssertion.Get(expr)
		self := source.LocationOf(val).Node
		val.X = xyz.New(escape.routeExpressionWithBuddy(val.X, self))
		if t, ok := val.Type.Get(); ok {
			val.Type = xyz.New(escape.RouteType(t))
		}
		return source.Expressions.TypeAssertion.New(val)
	case source.Expressions.Unary:
		val := source.Expressions.Unary.Get(expr)
		self := source.LocationOf(val).Node
		val.X = xyz.New(escape.routeExpressionWithBuddy(val.X, self))
		return source.Expressions.Unary.New(val)
	case source.Expressions.Expansion:
		val := source.Expressions.Expansion.Get(expr)
		if x, ok := val.Expression.Get(); ok {
			self := source.LocationOf(val).Node
			val.Expression = xyz.New(escape.routeExpressionWithBuddy(x, self))
		}
		return source.Expressions.Expansion.New(val)
	case source.Expressions.Constant:
		return expr
	case source.Expressions.Composite:
		val := source.Expressions.Composite.Get(expr)
		self := source.LocationOf(val).Node
		if t, ok := val.Type.Get(); ok {
			val.Type = xyz.New(escape.RouteType(t))
		}
		for i := range val.Elements {
			val.Elements[i] = escape.routeExpressionWithBuddy(val.Elements[i], self)
		}
		return source.Expressions.Composite.New(val)
	case source.Expressions.Function:
		val := source.Expressions.Function.Get(expr)
		val.Type = escape.RouteTypeFunction(val.Type)
		val.Body = escape.routeStatementBlockCtx(val.Body, routing{})
		return source.Expressions.Function.New(val)
	case source.Expressions.Type:
		val := source.Expressions.Type.Get(expr)
		val = escape.RouteType(val)
		return source.Expressions.Type.New(val)
	case source.Expressions.Nil:
		return expr
	case source.Expressions.BuiltinFunction:
		if parent != nil {
			val := source.Expressions.BuiltinFunction.Get(expr)
			val.Escapes = escape.Make(val.Node, false, []ast.Node{parent})
			return source.Expressions.BuiltinFunction.New(val)
		}
		return expr
	case source.Expressions.ImportedPackage:
		if parent != nil {
			val := source.Expressions.ImportedPackage.Get(expr)
			val.Escapes = escape.Make(val.Node, false, []ast.Node{parent})
			return source.Expressions.ImportedPackage.New(val)
		}
		return expr
	case source.Expressions.DefinedType:
		if parent != nil {
			val := source.Expressions.DefinedType.Get(expr)
			val.Escapes = escape.Make(val.Node, false, []ast.Node{parent})
			return source.Expressions.DefinedType.New(val)
		}
		return expr
	case source.Expressions.DefinedFunction:
		if parent != nil {
			val := source.Expressions.DefinedFunction.Get(expr)
			val.Escapes = escape.Make(val.Node, false, []ast.Node{parent})
			return source.Expressions.DefinedFunction.New(val)
		}
		return expr
	case source.Expressions.DefinedVariable:
		if parent != nil {
			val := source.Expressions.DefinedVariable.Get(expr)
			val.Escapes = escape.Make(val.Node, false, []ast.Node{parent})
			return source.Expressions.DefinedVariable.New(val)
		}
		return expr
	case source.Expressions.DefinedConstant:
		if parent != nil {
			val := source.Expressions.DefinedConstant.Get(expr)
			val.Escapes = escape.Make(val.Node, false, []ast.Node{parent})
			return source.Expressions.DefinedConstant.New(val)
		}
		return expr
	case source.Expressions.AwaitChannel:
		val := source.Expressions.AwaitChannel.Get(expr)
		self := source.LocationOf(val).Node
		val.Chan = xyz.New(escape.routeExpressionWithBuddy(val.Chan, self))
		return source.Expressions.AwaitChannel.New(val)
	case source.Expressions.FunctionCall:
		val := source.Expressions.FunctionCall.Get(expr)
		self := source.LocationOf(val).Node
		val.Function = xyz.New(escape.routeExpressionWithBuddy(val.Function, self))
		for i := range val.Arguments {
			val.Arguments[i] = escape.routeExpressionWithBuddy(val.Arguments[i], self)
		}
		return source.Expressions.FunctionCall.New(val)
	default:
		return expr
	}
}

func (escape graph) routeStatementBlock(block source.StatementBlock) source.StatementBlock {
	for i := range block.Statements {
		block.Statements[i] = escape.routeStatement(block.Statements[i])
	}
	return block
}

func (escape graph) routeStatementBlockCtx(block source.StatementBlock, ctx routing) source.StatementBlock {
	for i := range block.Statements {
		block.Statements[i] = escape.routeStatementCtx(block.Statements[i], ctx)
	}
	return block
}

func (escape graph) routeStatement(stmt source.Statement) source.Statement {
	return escape.routeStatementCtx(stmt, routing{})
}

func (escape graph) routeStatementCtx(stmt source.Statement, ctx routing) source.Statement {
	switch xyz.ValueOf(stmt) {
	case source.Statements.Bad:
		return stmt
	case source.Statements.Assignment:
		val := source.Statements.Assignment.Get(stmt)
		for i := range val.Variables {
			val.Variables[i] = escape.routeExpressionWithBuddy(val.Variables[i], nil)
		}
		for i := range val.Values {
			val.Values[i] = escape.routeExpressionWithBuddy(val.Values[i], nil)
		}
		return source.Statements.Assignment.New(val)
	case source.Statements.Block:
		val := source.Statements.Block.Get(stmt)
		val = escape.routeStatementBlockCtx(val, ctx)
		return source.Statements.Block.New(val)
	case source.Statements.Goto:
		return stmt
	case source.Statements.Definitions:
		val := source.Statements.Definitions.Get(stmt)
		for i := range val {
			escape.RouteDefinition(&val[i])
		}
		return source.Statements.Definitions.New(val)
	case source.Statements.Defer:
		val := source.Statements.Defer.Get(stmt)
		val.Call = escape.routeFunctionCallWithBuddy(val.Call, nil)
		return source.Statements.Defer.New(val)
	case source.Statements.Empty:
		return stmt
	case source.Statements.Expression:
		val := source.Statements.Expression.Get(stmt)
		val = escape.routeExpressionWithBuddy(val, nil)
		return source.Statements.Expression.New(val)
	case source.Statements.Go:
		val := source.Statements.Go.Get(stmt)
		goNode := source.LocationOf(val).Node
		_ = escape.Make(goNode, true, nil)
		val.Call = escape.routeFunctionCallWithBuddy(val.Call, goNode)
		return source.Statements.Go.New(val)
	case source.Statements.If:
		val := source.Statements.If.Get(stmt)
		if s, ok := val.Init.Get(); ok {
			val.Init = xyz.New(escape.routeStatementCtx(s, ctx))
		}
		val.Condition = xyz.New(escape.routeExpressionWithBuddy(val.Condition, nil))
		val.Body = escape.routeStatementBlockCtx(val.Body, ctx)
		if e, ok := val.Else.Get(); ok {
			val.Else = xyz.New(escape.routeStatementCtx(e, ctx))
		}
		return source.Statements.If.New(val)
	case source.Statements.For:
		val := source.Statements.For.Get(stmt)
		if s, ok := val.Init.Get(); ok {
			val.Init = xyz.New(escape.routeStatementCtx(s, ctx))
		}
		if e, ok := val.Condition.Get(); ok {
			val.Condition = xyz.New(escape.routeExpressionWithBuddy(e, nil))
		}
		if s, ok := val.Statement.Get(); ok {
			val.Statement = xyz.New(escape.routeStatementCtx(s, ctx))
		}
		val.Body = escape.routeStatementBlockCtx(val.Body, ctx)
		return source.Statements.For.New(val)
	case source.Statements.Increment:
		val := source.Statements.Increment.Get(stmt)
		val.WithLocation.Value = escape.routeExpressionWithBuddy(val.WithLocation.Value, nil)
		return source.Statements.Increment.New(val)
	case source.Statements.Decrement:
		val := source.Statements.Decrement.Get(stmt)
		val.WithLocation.Value = escape.routeExpressionWithBuddy(val.WithLocation.Value, nil)
		return source.Statements.Decrement.New(val)
	case source.Statements.Label:
		val := source.Statements.Label.Get(stmt)
		val.Statement = escape.routeStatementCtx(val.Statement, ctx)
		return source.Statements.Label.New(val)
	case source.Statements.Range:
		val := source.Statements.Range.Get(stmt)
		val.X = xyz.New(escape.routeExpressionWithBuddy(val.X, nil))
		val.Body = escape.routeStatementBlockCtx(val.Body, ctx)
		return source.Statements.Range.New(val)
	case source.Statements.Return:
		val := source.Statements.Return.Get(stmt)
		retNode := source.LocationOf(val).Node
		_ = escape.Make(retNode, true, nil)
		if len(val.Results) == 0 && len(ctx.namedResultBuddies) > 0 {
			for _, nameNode := range ctx.namedResultBuddies {
				_ = escape.Make(nameNode, false, []ast.Node{retNode})
			}
		}
		for i := range val.Results {
			val.Results[i] = escape.routeExpressionWithBuddy(val.Results[i], retNode)
		}
		return source.Statements.Return.New(val)
	case source.Statements.Select:
		val := source.Statements.Select.Get(stmt)
		for i := range val.Clauses {
			cl := &val.Clauses[i]
			if s, ok := cl.Statement.Get(); ok {
				cl.Statement = xyz.New(escape.routeStatementCtx(s, ctx))
			}
			for j := range cl.Body {
				cl.Body[j] = escape.routeStatementCtx(cl.Body[j], ctx)
			}
		}
		return source.Statements.Select.New(val)
	case source.Statements.Send:
		val := source.Statements.Send.Get(stmt)
		val.X = xyz.New(escape.routeExpressionWithBuddy(val.X, nil))
		val.Value = xyz.New(escape.routeExpressionWithBuddy(val.Value, nil))
		return source.Statements.Send.New(val)
	case source.Statements.SwitchType:
		val := source.Statements.SwitchType.Get(stmt)
		if s, ok := val.Init.Get(); ok {
			val.Init = xyz.New(escape.routeStatementCtx(s, ctx))
		}
		val.Assign = escape.routeStatementCtx(val.Assign, ctx)
		for i := range val.Claused {
			cc := &val.Claused[i]
			for j := range cc.Body {
				cc.Body[j] = escape.routeStatementCtx(cc.Body[j], ctx)
			}
		}
		return source.Statements.SwitchType.New(val)
	case source.Statements.Switch:
		val := source.Statements.Switch.Get(stmt)
		if s, ok := val.Init.Get(); ok {
			val.Init = xyz.New(escape.routeStatementCtx(s, ctx))
		}
		if e, ok := val.Value.Get(); ok {
			val.Value = xyz.New(escape.routeExpressionWithBuddy(e, nil))
		}
		for i := range val.Clauses {
			cc := &val.Clauses[i]
			for j := range cc.Expressions {
				cc.Expressions[j] = escape.routeExpressionWithBuddy(cc.Expressions[j], nil)
			}
			for j := range cc.Body {
				cc.Body[j] = escape.routeStatementCtx(cc.Body[j], ctx)
			}
		}
		return source.Statements.Switch.New(val)
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

func (escape graph) routeFunctionCall(call source.FunctionCall) source.FunctionCall {
	return escape.routeFunctionCallWithBuddy(call, nil)
}

func (escape graph) routeFunctionCallWithBuddy(call source.FunctionCall, buddy ast.Node) source.FunctionCall {
	self := source.LocationOf(call).Node
	call.Function = xyz.New(escape.routeExpressionWithBuddy(call.Function, self))
	for i := range call.Arguments {
		parent := self
		if buddy != nil {
			parent = buddy
		}
		call.Arguments[i] = escape.routeExpressionWithBuddy(call.Arguments[i], parent)
	}
	return call
}

func (escape graph) RouteType(t source.Type) source.Type {
	switch xyz.ValueOf(t) {
	case source.Types.Bad:
		return t
	case source.Types.Unknown:
		return t
	case source.Types.TypeNamed:
		return t
	case source.Types.Parenthesized:
		val := source.Types.Parenthesized.Get(t)
		val.X = xyz.New(escape.routeExpressionWithBuddy(val.X, nil))
		return source.Types.Parenthesized.New(val)
	case source.Types.Selection:
		val := source.Types.Selection.Get(t)
		val.X = xyz.New(escape.routeExpressionWithBuddy(val.X, nil))
		val.Selection = xyz.New(escape.routeExpressionWithBuddy(val.Selection, nil))
		return source.Types.Selection.New(val)
	case source.Types.TypeArray:
		val := source.Types.TypeArray.Get(t)
		if e, ok := val.Length.Get(); ok {
			val.Length = xyz.New(escape.routeExpressionWithBuddy(e, nil))
		}
		val.ElementType = escape.RouteType(val.ElementType)
		return source.Types.TypeArray.New(val)
	case source.Types.TypeChannel:
		val := source.Types.TypeChannel.Get(t)
		val.Value = xyz.New(escape.routeExpressionWithBuddy(val.Value, nil))
		return source.Types.TypeChannel.New(val)
	case source.Types.TypeFunction:
		return escape.RouteTypeFunction(source.Types.TypeFunction.Get(t))
	case source.Types.TypeInterface:
		val := source.Types.TypeInterface.Get(t)
		val.Methods = escape.routeFieldList(val.Methods)
		return source.Types.TypeInterface.New(val)
	case source.Types.TypeMap:
		val := source.Types.TypeMap.Get(t)
		val.Key = xyz.New(escape.routeExpressionWithBuddy(val.Key, nil))
		val.Value = xyz.New(escape.routeExpressionWithBuddy(val.Value, nil))
		return source.Types.TypeMap.New(val)
	case source.Types.TypeStruct:
		val := source.Types.TypeStruct.Get(t)
		val.Fields = escape.routeFieldList(val.Fields)
		return source.Types.TypeStruct.New(val)
	case source.Types.TypeVariadic:
		val := source.Types.TypeVariadic.Get(t)
		val.ElementType.Value = escape.RouteType(val.ElementType.Value)
		return source.Types.TypeVariadic.New(val)
	case source.Types.Pointer:
		val := source.Types.Pointer.Get(t)
		val.WithLocation.Value = escape.routeExpressionWithBuddy(val.WithLocation.Value, nil)
		return source.Types.Pointer.New(val)
	default:
		return t
	}
}

func (escape graph) RouteTypeFunction(tf source.TypeFunction) source.TypeFunction {
	if tf.TypeParams.Ok {
		params := tf.TypeParams.Get()
		params = escape.routeFieldList(params)
		tf.TypeParams = xyz.New(params)
	}
	tf.Arguments = escape.routeFieldList(tf.Arguments)
	if tf.Results.Ok {
		results := tf.Results.Get()
		results = escape.routeFieldList(results)
		tf.Results = xyz.New(results)
	}
	return tf
}

func (escape graph) routeFieldList(list source.FieldList) source.FieldList {
	for i := range list.Fields {
		escape.routeField(&list.Fields[i])
	}
	return list
}

func (escape graph) routeField(f *source.Field) {
	f.Type = escape.RouteType(f.Type)
	if f.Tag.Ok {
		tag := f.Tag.Get()
		f.Tag = xyz.New(tag)
	}
}
