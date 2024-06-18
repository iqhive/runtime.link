package source

import (
	"go/ast"
	"go/token"
	"io"

	"runtime.link/xyz"
)

type StatementGoto struct {
	Location
	Keyword WithLocation[token.Token]
	Label   xyz.Maybe[Identifier]
}

func (pkg *Package) loadStatementGoto(in *ast.BranchStmt) StatementGoto {
	var label xyz.Maybe[Identifier]
	if in.Label != nil {
		label = xyz.New(pkg.loadIdentifier(in.Label))
	}
	return StatementGoto{
		Keyword: WithLocation[token.Token]{Value: in.Tok, SourceLocation: pkg.location(in.TokPos)},
		Label:   label,
	}
}

func (stmt StatementGoto) compile(w io.Writer, tabs int) error {
	return stmt.Location.Errorf("goto is not supported in zgo yet")
}

type StatementLabel struct {
	Location
	Label     Identifier
	Colon     Location
	Statement Statement
}

func (pkg *Package) loadStatementLabel(in *ast.LabeledStmt) StatementLabel {
	return StatementLabel{
		Location:  pkg.locations(in.Pos(), in.End()),
		Label:     pkg.loadIdentifier(in.Label),
		Colon:     pkg.location(in.Colon),
		Statement: pkg.loadStatement(in.Stmt),
	}
}

func (stmt StatementLabel) compile(w io.Writer, tabs int) error {
	return stmt.Location.Errorf("labeled statements are not supported in zgo yet")
}
