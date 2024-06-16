package source

import (
	"fmt"
	"go/ast"
	"io"
)

type StatementReturn struct {
	Keyword Location
	Results []Expression
}

func (pkg *Package) loadStatementReturn(in *ast.ReturnStmt) StatementReturn {
	var results []Expression
	for _, expr := range in.Results {
		results = append(results, pkg.loadExpression(expr))
	}
	return StatementReturn{
		Keyword: Location(in.Return),
		Results: results,
	}
}

func (stmt StatementReturn) compile(w io.Writer) error {
	fmt.Fprintf(w, "return")
	for _, result := range stmt.Results {
		fmt.Fprintf(w, " ")
		if err := result.compile(w); err != nil {
			return err
		}
	}
	return nil
}
