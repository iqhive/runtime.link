package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"

	"runtime.link/xyz"
)

type Statement xyz.Switch[Node, struct {
	Bad         xyz.Case[Statement, Bad]
	Assignment  xyz.Case[Statement, StatementAssignment]
	Block       xyz.Case[Statement, StatementBlock]
	Branch      xyz.Case[Statement, StatementBranch]
	Declaration xyz.Case[Statement, Declaration]
	Defer       xyz.Case[Statement, StatementDefer]
	Empty       xyz.Case[Statement, StatementEmpty]
	Expression  xyz.Case[Statement, Expression]
	Go          xyz.Case[Statement, StatementGo]
	If          xyz.Case[Statement, StatementIf]
	For         xyz.Case[Statement, StatementFor]
	Increment   xyz.Case[Statement, Star]
	Decrement   xyz.Case[Statement, Star]
	Labeled     xyz.Case[Statement, StatementLabeled]
	Range       xyz.Case[Statement, StatementRange]
	Return      xyz.Case[Statement, StatementReturn]
	Select      xyz.Case[Statement, StatementSelect]
	Send        xyz.Case[Statement, StatementSend]
	Switch      xyz.Case[Statement, StatementSwitch]
	SwitchType  xyz.Case[Statement, StatementSwitchType]
}]

var Statements = xyz.AccessorFor(Statement.Values)

func (pkg *Package) loadStatement(node ast.Node) Statement {
	switch stmt := node.(type) {
	case *ast.BadStmt:
		return Statements.Bad.New(pkg.loadBad(stmt, stmt.From, stmt.To))
	case *ast.AssignStmt:
		return Statements.Assignment.New(pkg.loadStatementAssignment(stmt))
	case *ast.BlockStmt:
		return Statements.Block.New(pkg.loadStatementBlock(stmt))
	case *ast.BranchStmt:
		return Statements.Branch.New(pkg.loadStatementBranch(stmt))
	case *ast.DeclStmt:
		return Statements.Declaration.New(pkg.loadDeclaration(stmt.Decl))
	case *ast.DeferStmt:
		return Statements.Defer.New(pkg.loadStatementDefer(stmt))
	case *ast.EmptyStmt:
		return Statements.Empty.New(pkg.loadStatementEmpty(stmt))
	case *ast.ExprStmt:
		return Statements.Expression.New(pkg.loadExpression(stmt.X))
	case *ast.GoStmt:
		return Statements.Go.New(pkg.loadStatementGo(stmt))
	case *ast.IfStmt:
		return Statements.If.New(pkg.loadStatementIf(stmt))
	case *ast.IncDecStmt:
		if stmt.Tok == token.INC {
			return Statements.Increment.New(Star{
				WithLocation: WithLocation[Expression]{
					Value:          pkg.loadExpression(stmt.X),
					SourceLocation: Location(stmt.TokPos),
				},
			})
		}
		return Statements.Decrement.New(Star{
			WithLocation: WithLocation[Expression]{
				Value:          pkg.loadExpression(stmt.X),
				SourceLocation: Location(stmt.TokPos),
			},
		})
	case *ast.LabeledStmt:
		return Statements.Labeled.New(pkg.loadStatementLabeled(stmt))
	case *ast.ForStmt:
		return Statements.For.New(pkg.loadStatementFor(stmt))
	case *ast.RangeStmt:
		return Statements.Range.New(pkg.loadStatementRange(stmt))
	case *ast.ReturnStmt:
		return Statements.Return.New(pkg.loadStatementReturn(stmt))
	case *ast.SelectStmt:
		return Statements.Select.New(pkg.loadStatementSelect(stmt))
	case *ast.SendStmt:
		return Statements.Send.New(pkg.loadStatementSend(stmt))
	case *ast.SwitchStmt:
		return Statements.Switch.New(pkg.loadStatementSwitch(stmt))
	case *ast.TypeSwitchStmt:
		return Statements.SwitchType.New(pkg.loadStatementSwitchType(stmt))
	default:
		panic("unexpected statement type " + fmt.Sprintf("%T", stmt))
	}
}

func (stmt Statement) compile(w io.Writer) error {
	if xyz.ValueOf(stmt) == Statements.Expression {
		expr := Statements.Expression.Get(stmt)
		switch expr := expr.TypeAndValue().Type.(type) {
		case *types.Basic:
			fmt.Fprintf(w, "_ = ")
		case *types.Tuple:
			if expr.Len() == 0 {
				break
			}
			return fmt.Errorf("unsupported expression type %T", expr)
		default:
			return fmt.Errorf("unsupported expression type %T", expr)
		}
	}
	value, _ := stmt.Get()
	if err := value.compile(w); err != nil {
		return err
	}
	fmt.Fprintf(w, ";\n")
	return nil
}

type StatementBlock struct {
	Opening    Location
	Statements []Statement
	Closing    Location
}

func (pkg *Package) loadStatementBlock(in *ast.BlockStmt) StatementBlock {
	var out StatementBlock

	out.Opening = Location(in.Lbrace)
	for _, stmt := range in.List {
		out.Statements = append(out.Statements, pkg.loadStatement(stmt))
	}
	out.Closing = Location(in.Rbrace)
	return out
}

type StatementBranch struct {
	Keyword WithLocation[token.Token]
	Label   xyz.Maybe[Identifier]
}

func (pkg *Package) loadStatementBranch(in *ast.BranchStmt) StatementBranch {
	var label xyz.Maybe[Identifier]
	if in.Label != nil {
		label = xyz.New(pkg.loadIdentifier(in.Label))
	}
	return StatementBranch{

		Keyword: WithLocation[token.Token]{Value: in.Tok, SourceLocation: Location(in.TokPos)},
		Label:   label,
	}
}

type StatementDefer struct {
	Keyword Location
	Call    ExpressionCall
}

func (pkg *Package) loadStatementDefer(in *ast.DeferStmt) StatementDefer {
	return StatementDefer{

		Keyword: Location(in.Defer),
		Call:    pkg.loadExpressionCall(in.Call),
	}
}

type StatementEmpty struct {
	Semicolon Location
	Implicit  bool
}

func (pkg *Package) loadStatementEmpty(in *ast.EmptyStmt) StatementEmpty {
	return StatementEmpty{
		Semicolon: Location(in.Semicolon),
		Implicit:  in.Implicit,
	}
}

type StatementFor struct {
	Keyword   Location
	Init      xyz.Maybe[Statement]
	Condition xyz.Maybe[Expression]
	Statement xyz.Maybe[Statement]
	Body      StatementBlock
}

func (pkg *Package) loadStatementFor(in *ast.ForStmt) StatementFor {
	return StatementFor{
		Keyword:   Location(in.For),
		Init:      xyz.New(pkg.loadStatement(in.Init)),
		Condition: xyz.New(pkg.loadExpression(in.Cond)),
		Statement: xyz.New(pkg.loadStatement(in.Post)),
		Body:      pkg.loadStatementBlock(in.Body),
	}
}

type StatementGo struct {
	Keyword Location
	Call    ExpressionCall
}

func (pkg *Package) loadStatementGo(in *ast.GoStmt) StatementGo {
	return StatementGo{

		Keyword: Location(in.Go),
		Call:    pkg.loadExpressionCall(in.Call),
	}
}

type StatementIf struct {
	Keyword   Location
	Init      xyz.Maybe[Statement]
	Condition Expression
	Body      StatementBlock
	Else      xyz.Maybe[Statement]
}

func (pkg *Package) loadStatementIf(in *ast.IfStmt) StatementIf {
	return StatementIf{
		Keyword:   Location(in.If),
		Init:      xyz.New(pkg.loadStatement(in.Init)),
		Condition: pkg.loadExpression(in.Cond),
		Body:      pkg.loadStatementBlock(in.Body),
		Else:      xyz.New(pkg.loadStatement(in.Else)),
	}
}

type StatementLabeled struct {
	Label     Identifier
	Colon     Location
	Statement Statement
}

func (pkg *Package) loadStatementLabeled(in *ast.LabeledStmt) StatementLabeled {
	return StatementLabeled{
		Label:     pkg.loadIdentifier(in.Label),
		Colon:     Location(in.Colon),
		Statement: pkg.loadStatement(in.Stmt),
	}
}

type StatementRange struct {
	For        Location
	Key, Value Expression
	Token      WithLocation[token.Token]
	Keyword    Location
	X          Expression
	Body       StatementBlock
}

func (pkg *Package) loadStatementRange(in *ast.RangeStmt) StatementRange {
	return StatementRange{

		For:     Location(in.For),
		Key:     pkg.loadExpression(in.Key),
		Value:   pkg.loadExpression(in.Value),
		Token:   WithLocation[token.Token]{Value: in.Tok, SourceLocation: Location(in.TokPos)},
		Keyword: Location(in.Range),
		X:       pkg.loadExpression(in.X),
		Body:    pkg.loadStatementBlock(in.Body),
	}
}

type StatementSelect struct {
	Keyword Location
	Clauses []SelectCaseClause
}

func (pkg *Package) loadStatementSelect(in *ast.SelectStmt) StatementSelect {
	var clauses []SelectCaseClause
	for _, clause := range in.Body.List {
		clauses = append(clauses, pkg.loadSelectCaseClause(clause.(*ast.CommClause)))
	}
	return StatementSelect{

		Keyword: Location(in.Select),
		Clauses: clauses,
	}
}

type StatementSend struct {
	X     Expression
	Arrow Location
	Value Expression
}

func (pkg *Package) loadStatementSend(in *ast.SendStmt) StatementSend {
	return StatementSend{
		X:     pkg.loadExpression(in.Chan),
		Arrow: Location(in.Arrow),
		Value: pkg.loadExpression(in.Value),
	}
}

type StatementSwitch struct {
	Keyword Location
	Init    Statement
	Value   xyz.Maybe[Expression]
	Clauses []SwitchCaseClause
}

func (pkg *Package) loadStatementSwitch(in *ast.SwitchStmt) StatementSwitch {
	var value xyz.Maybe[Expression]
	if in.Tag != nil {
		value = xyz.New(pkg.loadExpression(in.Tag))
	}
	var clauses []SwitchCaseClause
	for _, clause := range in.Body.List {
		clauses = append(clauses, pkg.loadSwitchCaseClause(clause.(*ast.CaseClause)))
	}
	return StatementSwitch{

		Keyword: Location(in.Switch),
		Init:    pkg.loadStatement(in.Init),
		Value:   value,
		Clauses: clauses,
	}
}

type StatementSwitchType struct {
	Keyword Location
	Init    Statement
	Assign  Statement
	Claused []SwitchCaseClause
}

func (pkg *Package) loadStatementSwitchType(in *ast.TypeSwitchStmt) StatementSwitchType {
	var clauses []SwitchCaseClause
	for _, clause := range in.Body.List {
		clauses = append(clauses, pkg.loadSwitchCaseClause(clause.(*ast.CaseClause)))
	}
	return StatementSwitchType{

		Keyword: Location(in.Switch),
		Init:    pkg.loadStatement(in.Init),
		Assign:  pkg.loadStatement(in.Assign),
		Claused: clauses,
	}
}
