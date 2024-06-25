package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"strings"

	"runtime.link/xyz"
)

type Statement xyz.Switch[Node, struct {
	Bad         xyz.Case[Statement, Bad]
	Assignment  xyz.Case[Statement, StatementAssignment]
	Block       xyz.Case[Statement, StatementBlock]
	Goto        xyz.Case[Statement, StatementGoto]
	Declaration xyz.Case[Statement, Declaration]
	Defer       xyz.Case[Statement, StatementDefer]
	Empty       xyz.Case[Statement, StatementEmpty]
	Expression  xyz.Case[Statement, Expression]
	Go          xyz.Case[Statement, StatementGo]
	If          xyz.Case[Statement, StatementIf]
	For         xyz.Case[Statement, StatementFor]
	Increment   xyz.Case[Statement, StatementIncrement]
	Decrement   xyz.Case[Statement, StatementDecrement]
	Label       xyz.Case[Statement, StatementLabel]
	Range       xyz.Case[Statement, StatementRange]
	Return      xyz.Case[Statement, StatementReturn]
	Select      xyz.Case[Statement, StatementSelect]
	Send        xyz.Case[Statement, StatementSend]
	Switch      xyz.Case[Statement, StatementSwitch]
	SwitchType  xyz.Case[Statement, StatementSwitchType]

	Continue    xyz.Case[Statement, StatementContinue]
	Break       xyz.Case[Statement, StatementBreak]
	Fallthrough xyz.Case[Statement, StatementFallthrough]
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
		switch stmt.Tok {
		case token.CONTINUE:
			return Statements.Continue.New(pkg.loadStatementContinue(stmt))
		case token.GOTO:
			return Statements.Goto.New(pkg.loadStatementGoto(stmt))
		case token.BREAK:
			return Statements.Break.New(pkg.loadStatementBreak(stmt))
		case token.FALLTHROUGH:
			return Statements.Fallthrough.New(pkg.loadStatementFallthrough(stmt))
		default:
			panic("unexpected branch statement " + fmt.Sprintf("%T %s", node, stmt.Tok))
		}
	case *ast.DeclStmt:
		return Statements.Declaration.New(pkg.loadDeclaration(stmt.Decl, false))
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
			return Statements.Increment.New(StatementIncrement(Star{
				WithLocation: WithLocation[Expression]{
					Value:          pkg.loadExpression(stmt.X),
					SourceLocation: pkg.location(stmt.TokPos),
				},
			}))
		}
		return Statements.Decrement.New(StatementDecrement(Star{
			WithLocation: WithLocation[Expression]{
				Value:          pkg.loadExpression(stmt.X),
				SourceLocation: pkg.location(stmt.TokPos),
			},
		}))
	case *ast.LabeledStmt:
		switch labeled := stmt.Stmt.(type) {
		case *ast.RangeStmt:
			loop := pkg.loadStatementRange(labeled)
			loop.Label = stmt.Label.Name
			return Statements.Range.New(loop)
		case *ast.ForStmt:
			loop := pkg.loadStatementFor(labeled)
			loop.Label = stmt.Label.Name
			return Statements.For.New(loop)
		}
		return Statements.Label.New(pkg.loadStatementLabel(stmt))
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
		panic("unexpected statement type " + fmt.Sprintf("%T", node))
	}
}

func (stmt Statement) sources() Location {
	value, _ := stmt.Get()
	return value.sources()
}

func (stmt Statement) compile(w io.Writer, tabs int) error {
	switch xyz.ValueOf(stmt) {
	case Statements.Declaration:
	default:
		if tabs >= 0 {
			fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
		}
	}
	if tabs < 0 {
		tabs = -tabs
	}
	if xyz.ValueOf(stmt) == Statements.Expression {
		expr := Statements.Expression.Get(stmt)
		switch expr := expr.TypeAndValue().Type.(type) {
		case *types.Basic:
			fmt.Fprintf(w, "_ = ")
		case *types.Tuple:
			if expr.Len() == 0 {
				break
			}
			for i := 0; i < expr.Len(); i++ {
				if i > 0 {
					fmt.Fprintf(w, ", ")
				}
				fmt.Fprintf(w, "_")
			}
			fmt.Fprintf(w, " = ")
		default:
			return fmt.Errorf("unsupported expression type %T", expr)
		}
	}
	value, _ := stmt.Get()
	if err := value.compile(w, tabs); err != nil {
		return err
	}
	switch xyz.ValueOf(stmt) {
	case Statements.Block, Statements.Empty, Statements.For, Statements.Range, Statements.If, Statements.Declaration:
		return nil
	default:
		fmt.Fprintf(w, ";")
		return nil
	}
}
