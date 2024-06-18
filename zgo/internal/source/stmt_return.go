package source

import (
	"fmt"
	"go/ast"
	"io"
)

type StatementReturn struct {
	Location
	Keyword Location
	Results []Expression
}

func (pkg *Package) loadStatementReturn(in *ast.ReturnStmt) StatementReturn {
	var results []Expression
	for _, expr := range in.Results {
		results = append(results, pkg.loadExpression(expr))
	}
	return StatementReturn{
		Location: pkg.locations(in.Pos(), in.End()),
		Keyword:  pkg.location(in.Return),
		Results:  results,
	}
}

func (stmt StatementReturn) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "return")
	for _, result := range stmt.Results {
		fmt.Fprintf(w, " ")
		if err := result.compile(w, tabs); err != nil {
			return err
		}
	}
	return nil
}
