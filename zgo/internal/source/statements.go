package source

import (
	"go/token"

	"runtime.link/xyz"
)

type Statement xyz.Tagged[Node, struct {
	Bad         xyz.Case[Statement, Bad]
	Assignment  xyz.Case[Statement, StatementAssignment]
	Block       xyz.Case[Statement, StatementBlock]
	Goto        xyz.Case[Statement, StatementGoto]
	Definitions xyz.Case[Statement, StatementDefinitions]
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

func (stmt Statement) sources() Location {
	value, _ := stmt.Get()
	return value.sources()
}

type StatementDefinitions []Definition

func (stmt StatementDefinitions) sources() Location {
	if len(stmt) == 0 {
		return Location{}
	}
	return stmt[0].sources()
}

// {}
type StatementBlock struct {
	Location

	Opening    Location
	Statements []Statement
	Closing    Location
}

// X--
type StatementDecrement Star

// defer Call(...Call.Arguments)
type StatementDefer struct {
	Location

	Keyword Location
	Call    FunctionCall

	OutermostScope bool
}

// ;
type StatementEmpty struct {
	Location
	Semicolon Location
	Implicit  bool
}

// for Init; Condition; Statement { Body... }
type StatementFor struct {
	Location
	Keyword   Location
	Label     string
	Init      xyz.Maybe[Statement]
	Condition xyz.Maybe[Expression]
	Statement xyz.Maybe[Statement]
	Body      StatementBlock
}

// for Key,Value := range X { Body... }
type StatementRange struct {
	Location

	Label string

	For     Location
	Key     xyz.Maybe[DefinedVariable]
	Value   xyz.Maybe[DefinedVariable]
	Token   WithLocation[token.Token]
	Keyword Location
	X       Expression
	Body    StatementBlock
}

// continue Label
type StatementContinue struct {
	Location

	Label xyz.Maybe[Identifier]
}

// break Label
type StatementBreak struct {
	Location

	Label xyz.Maybe[Identifier]
}

// go Call(...Call.Arguments)
type StatementGo struct {
	Location
	Keyword Location
	Call    FunctionCall
}

// goto StatementGoto
type StatementGoto struct {
	Location
	Keyword WithLocation[token.Token]
	Label   xyz.Maybe[Identifier]
}

// Label: Statement
type StatementLabel struct {
	Location
	Label     Identifier
	Colon     Location
	Statement Statement
}

// if Init; Condition { Body... } else { Else... }
type StatementIf struct {
	Location
	Keyword   Location
	Init      xyz.Maybe[Statement]
	Condition Expression
	Body      StatementBlock
	Else      xyz.Maybe[Statement]
}

// X++
type StatementIncrement Star

// return Results...
type StatementReturn struct {
	Location
	Keyword Location
	Results []Expression
}

// select { Clauses... }
type StatementSelect struct {
	Location
	Keyword Location
	Clauses []SelectCaseClause
}

// case Statement: Body...
type SelectCaseClause struct {
	Location

	Keyword   Location
	Statement xyz.Maybe[Statement]
	Colon     Location
	Body      []Statement
}

// X <- Value
type StatementSend struct {
	Location
	X     Expression
	Arrow Location
	Value Expression
}

// switch Init; Value.(type) { Clauses... }
type StatementSwitchType struct {
	Location
	Keyword Location
	Init    xyz.Maybe[Statement]
	Assign  Statement
	Claused []SwitchCaseClause
}

// switch Init; Value { Clauses... }
type StatementSwitch struct {
	Location
	Keyword Location
	Init    xyz.Maybe[Statement]
	Value   xyz.Maybe[Expression]
	Clauses []SwitchCaseClause
}

// case Expression: Body...
type SwitchCaseClause struct {
	Location

	Keyword     Location
	Expressions []Expression
	Colon       Location
	Body        []Statement

	Fallsthrough bool
}

// fallthrough
type StatementFallthrough struct {
	Location
}

// Variables... = Values...
type StatementAssignment struct {
	Location
	Variables []Expression
	Token     WithLocation[token.Token]
	Values    []Expression
}
