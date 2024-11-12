package parser

import (
	"fmt"
	"go/ast"
	"go/token"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func loadStatement(pkg *source.Package, node ast.Node) source.Statement {
	switch stmt := node.(type) {
	case *ast.BadStmt:
		return source.Statements.Bad.New(loadBad(pkg, stmt, stmt.From, stmt.To))
	case *ast.AssignStmt:
		return source.Statements.Assignment.New(loadStatementAssignment(pkg, stmt))
	case *ast.BlockStmt:
		return source.Statements.Block.New(loadStatementBlock(pkg, stmt))
	case *ast.BranchStmt:
		switch stmt.Tok {
		case token.CONTINUE:
			return source.Statements.Continue.New(loadStatementContinue(pkg, stmt))
		case token.GOTO:
			return source.Statements.Goto.New(loadStatementGoto(pkg, stmt))
		case token.BREAK:
			return source.Statements.Break.New(loadStatementBreak(pkg, stmt))
		case token.FALLTHROUGH:
			return source.Statements.Fallthrough.New(loadStatementFallthrough(pkg, stmt))
		default:
			panic("unexpected branch statement " + fmt.Sprintf("%T %s", node, stmt.Tok))
		}
	case *ast.DeclStmt:
		return source.Statements.Declaration.New(loadDeclaration(pkg, stmt.Decl, false))
	case *ast.DeferStmt:
		return source.Statements.Defer.New(loadStatementDefer(pkg, stmt))
	case *ast.EmptyStmt:
		return source.Statements.Empty.New(loadStatementEmpty(pkg, stmt))
	case *ast.ExprStmt:
		return source.Statements.Expression.New(loadExpression(pkg, stmt.X))
	case *ast.GoStmt:
		return source.Statements.Go.New(loadStatementGo(pkg, stmt))
	case *ast.IfStmt:
		return source.Statements.If.New(loadStatementIf(pkg, stmt))
	case *ast.IncDecStmt:
		if stmt.Tok == token.INC {
			return source.Statements.Increment.New(source.StatementIncrement(source.Star{
				WithLocation: source.WithLocation[source.Expression]{
					Value:          loadExpression(pkg, stmt.X),
					SourceLocation: locationIn(pkg, stmt.TokPos),
				},
			}))
		}
		return source.Statements.Decrement.New(source.StatementDecrement(source.Star{
			WithLocation: source.WithLocation[source.Expression]{
				Value:          loadExpression(pkg, stmt.X),
				SourceLocation: locationIn(pkg, stmt.TokPos),
			},
		}))
	case *ast.LabeledStmt:
		switch labeled := stmt.Stmt.(type) {
		case *ast.RangeStmt:
			loop := loadStatementRange(pkg, labeled)
			loop.Label = stmt.Label.Name
			return source.Statements.Range.New(loop)
		case *ast.ForStmt:
			loop := loadStatementFor(pkg, labeled)
			loop.Label = stmt.Label.Name
			return source.Statements.For.New(loop)
		}
		return source.Statements.Label.New(loadStatementLabel(pkg, stmt))
	case *ast.ForStmt:
		return source.Statements.For.New(loadStatementFor(pkg, stmt))
	case *ast.RangeStmt:
		return source.Statements.Range.New(loadStatementRange(pkg, stmt))
	case *ast.ReturnStmt:
		return source.Statements.Return.New(loadStatementReturn(pkg, stmt))
	case *ast.SelectStmt:
		return source.Statements.Select.New(loadStatementSelect(pkg, stmt))
	case *ast.SendStmt:
		return source.Statements.Send.New(loadStatementSend(pkg, stmt))
	case *ast.SwitchStmt:
		return source.Statements.Switch.New(loadStatementSwitch(pkg, stmt))
	case *ast.TypeSwitchStmt:
		return source.Statements.SwitchType.New(loadStatementSwitchType(pkg, stmt))
	default:
		panic("unexpected statement type " + fmt.Sprintf("%T", node))
	}
}

func loadStatementBlock(pkg *source.Package, in *ast.BlockStmt) source.StatementBlock {
	var out source.StatementBlock
	out.Location = locationRangeIn(pkg, in.Pos(), in.End())
	out.Opening = locationIn(pkg, in.Lbrace)
	for _, stmt := range in.List {
		out.Statements = append(out.Statements, loadStatement(pkg, stmt))
	}
	out.Closing = locationIn(pkg, in.Rbrace)
	return out
}

func loadStatementDefer(pkg *source.Package, in *ast.DeferStmt) source.StatementDefer {
	return source.StatementDefer{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		Keyword:  locationIn(pkg, in.Defer),
		Call:     loadExpressionCall(pkg, in.Call),
	}
}

func loadStatementEmpty(pkg *source.Package, in *ast.EmptyStmt) source.StatementEmpty {
	return source.StatementEmpty{
		Location:  locationRangeIn(pkg, in.Pos(), in.End()),
		Semicolon: locationIn(pkg, in.Semicolon),
		Implicit:  in.Implicit,
	}
}

func loadStatementFor(pkg *source.Package, in *ast.ForStmt) source.StatementFor {
	var init xyz.Maybe[source.Statement]
	if in.Init != nil {
		init = xyz.New(loadStatement(pkg, in.Init))
	}
	var cond xyz.Maybe[source.Expression]
	if in.Cond != nil {
		cond = xyz.New(loadExpression(pkg, in.Cond))
	}
	var stmt xyz.Maybe[source.Statement]
	if in.Post != nil {
		stmt = xyz.New(loadStatement(pkg, in.Post))
	}
	return source.StatementFor{
		Location:  locationRangeIn(pkg, in.Pos(), in.End()),
		Keyword:   locationIn(pkg, in.For),
		Init:      init,
		Condition: cond,
		Statement: stmt,
		Body:      loadStatementBlock(pkg, in.Body),
	}
}

func loadStatementRange(pkg *source.Package, in *ast.RangeStmt) source.StatementRange {
	var key xyz.Maybe[source.Identifier]
	if in.Key != nil {
		key = xyz.New(loadIdentifier(pkg, in.Key.(*ast.Ident)))
	}
	var val xyz.Maybe[source.Identifier]
	if in.Value != nil {
		val = xyz.New(loadIdentifier(pkg, in.Value.(*ast.Ident)))
	}
	return source.StatementRange{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		For:      locationIn(pkg, in.For),
		Key:      key,
		Value:    val,
		Token:    source.WithLocation[token.Token]{Value: in.Tok, SourceLocation: locationIn(pkg, in.TokPos)},
		Keyword:  locationIn(pkg, in.Range),
		X:        loadExpression(pkg, in.X),
		Body:     loadStatementBlock(pkg, in.Body),
	}
}

func loadStatementContinue(pkg *source.Package, in *ast.BranchStmt) source.StatementContinue {
	var label xyz.Maybe[source.Identifier]
	if in.Label != nil {
		label = xyz.New(loadIdentifier(pkg, in.Label))
	}
	return source.StatementContinue{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		Label:    label,
	}
}

func loadStatementBreak(pkg *source.Package, in *ast.BranchStmt) source.StatementBreak {
	var label xyz.Maybe[source.Identifier]
	if in.Label != nil {
		label = xyz.New(loadIdentifier(pkg, in.Label))
	}
	return source.StatementBreak{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		Label:    label,
	}
}

func loadStatementGo(pkg *source.Package, in *ast.GoStmt) source.StatementGo {
	call := loadExpressionCall(pkg, in.Call)
	call.Go = true
	return source.StatementGo{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		Keyword:  locationIn(pkg, in.Go),
		Call:     call,
	}
}

func loadStatementGoto(pkg *source.Package, in *ast.BranchStmt) source.StatementGoto {
	var label xyz.Maybe[source.Identifier]
	if in.Label != nil {
		label = xyz.New(loadIdentifier(pkg, in.Label))
	}
	return source.StatementGoto{
		Keyword: source.WithLocation[token.Token]{Value: in.Tok, SourceLocation: locationIn(pkg, in.TokPos)},
		Label:   label,
	}
}

func loadStatementLabel(pkg *source.Package, in *ast.LabeledStmt) source.StatementLabel {
	return source.StatementLabel{
		Location:  locationRangeIn(pkg, in.Pos(), in.End()),
		Label:     loadIdentifier(pkg, in.Label),
		Colon:     locationIn(pkg, in.Colon),
		Statement: loadStatement(pkg, in.Stmt),
	}
}

func loadStatementIf(pkg *source.Package, in *ast.IfStmt) source.StatementIf {
	var init xyz.Maybe[source.Statement]
	if in.Init != nil {
		init = xyz.New(loadStatement(pkg, in.Init))
	}
	var Else xyz.Maybe[source.Statement]
	if in.Else != nil {
		Else = xyz.New(loadStatement(pkg, in.Else))
	}
	return source.StatementIf{
		Location:  locationRangeIn(pkg, in.Pos(), in.End()),
		Keyword:   locationIn(pkg, in.If),
		Init:      init,
		Condition: loadExpression(pkg, in.Cond),
		Body:      loadStatementBlock(pkg, in.Body),
		Else:      Else,
	}
}

func loadStatementReturn(pkg *source.Package, in *ast.ReturnStmt) source.StatementReturn {
	var results []source.Expression
	for _, expr := range in.Results {
		results = append(results, loadExpression(pkg, expr))
	}
	return source.StatementReturn{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		Keyword:  locationIn(pkg, in.Return),
		Results:  results,
	}
}

func loadStatementSelect(pkg *source.Package, in *ast.SelectStmt) source.StatementSelect {
	var clauses []source.SelectCaseClause
	for _, clause := range in.Body.List {
		clauses = append(clauses, loadSelectCaseClause(pkg, clause.(*ast.CommClause)))
	}
	return source.StatementSelect{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		Keyword:  locationIn(pkg, in.Select),
		Clauses:  clauses,
	}
}

func loadSelectCaseClause(pkg *source.Package, in *ast.CommClause) source.SelectCaseClause {
	var out source.SelectCaseClause
	out.Location = locationRangeIn(pkg, in.Pos(), in.End())
	out.Keyword = locationIn(pkg, in.Case)
	if in.Comm != nil {
		out.Statement = xyz.New(loadStatement(pkg, in.Comm))
	}
	out.Colon = locationIn(pkg, in.Colon)
	for _, stmt := range in.Body {
		out.Body = append(out.Body, loadStatement(pkg, stmt))
	}
	return out
}

func loadStatementSend(pkg *source.Package, in *ast.SendStmt) source.StatementSend {
	return source.StatementSend{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		X:        loadExpression(pkg, in.Chan),
		Arrow:    locationIn(pkg, in.Arrow),
		Value:    loadExpression(pkg, in.Value),
	}
}

func loadStatementSwitchType(pkg *source.Package, in *ast.TypeSwitchStmt) source.StatementSwitchType {
	var clauses []source.SwitchCaseClause
	for _, clause := range in.Body.List {
		clauses = append(clauses, loadSwitchCaseClause(pkg, clause.(*ast.CaseClause)))
	}
	var init xyz.Maybe[source.Statement]
	if in.Init != nil {
		init = xyz.New(loadStatement(pkg, in.Init))
	}
	return source.StatementSwitchType{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		Keyword:  locationIn(pkg, in.Switch),
		Init:     init,
		Assign:   loadStatement(pkg, in.Assign),
		Claused:  clauses,
	}
}

func loadStatementSwitch(pkg *source.Package, in *ast.SwitchStmt) source.StatementSwitch {
	var value xyz.Maybe[source.Expression]
	if in.Tag != nil {
		value = xyz.New(loadExpression(pkg, in.Tag))
	}
	var clauses []source.SwitchCaseClause
	for _, clause := range in.Body.List {
		clauses = append(clauses, loadSwitchCaseClause(pkg, clause.(*ast.CaseClause)))
	}
	var init xyz.Maybe[source.Statement]
	if in.Init != nil {
		init = xyz.New(loadStatement(pkg, in.Init))
	}
	return source.StatementSwitch{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
		Keyword:  locationIn(pkg, in.Switch),
		Init:     init,
		Value:    value,
		Clauses:  clauses,
	}
}

func loadSwitchCaseClause(pkg *source.Package, in *ast.CaseClause) source.SwitchCaseClause {
	var out source.SwitchCaseClause
	out.Location = locationRangeIn(pkg, in.Pos(), in.End())
	out.Keyword = locationIn(pkg, in.Case)
	for _, expr := range in.List {
		out.Expressions = append(out.Expressions, loadExpression(pkg, expr))
	}
	out.Colon = locationIn(pkg, in.Colon)
	for _, stmt := range in.Body {
		stmt := loadStatement(pkg, stmt)
		if xyz.ValueOf(stmt) == source.Statements.Fallthrough {
			out.Fallsthrough = true
			break
		}
		out.Body = append(out.Body, stmt)
	}
	return out
}

func loadStatementFallthrough(pkg *source.Package, in *ast.BranchStmt) source.StatementFallthrough {
	return source.StatementFallthrough{
		Location: locationRangeIn(pkg, in.Pos(), in.End()),
	}
}

func loadStatementAssignment(pkg *source.Package, in *ast.AssignStmt) source.StatementAssignment {
	var out source.StatementAssignment
	out.Location = locationRangeIn(pkg, in.Pos(), in.End())
	for _, expr := range in.Lhs {
		out.Variables = append(out.Variables, loadExpression(pkg, expr))
	}
	out.Token = source.WithLocation[token.Token]{
		Value:          in.Tok,
		SourceLocation: locationIn(pkg, in.TokPos),
	}
	for _, expr := range in.Rhs {
		out.Values = append(out.Values, loadExpression(pkg, expr))
	}
	return out
}
