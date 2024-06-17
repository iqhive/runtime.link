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

type StatementAssignment struct {
	Variables []Expression
	Token     WithLocation[token.Token]
	Values    []Expression
}

func (pkg *Package) loadStatementAssignment(in *ast.AssignStmt) StatementAssignment {
	var out StatementAssignment
	for _, expr := range in.Lhs {
		out.Variables = append(out.Variables, pkg.loadExpression(expr))
	}
	out.Token = WithLocation[token.Token]{
		Value:          in.Tok,
		SourceLocation: Location(in.TokPos),
	}
	for _, expr := range in.Rhs {
		out.Values = append(out.Values, pkg.loadExpression(expr))
	}
	return out
}

func (stmt StatementAssignment) compile(w io.Writer) error {
	for i, variable := range stmt.Variables {
		switch xyz.ValueOf(variable) {
		case Expressions.Index:
			expr := Expressions.Index.Get(variable)
			if mtype, ok := expr.X.TypeAndValue().Type.(*types.Map); ok {
				if mtype.Key().String() == "string" {
					if err := expr.X.compile(w); err != nil {
						return err
					}
					fmt.Fprintf(w, ".set(go,")
					if err := expr.Index.compile(w); err != nil {
						return err
					}
					fmt.Fprintf(w, ", ")
					if err := stmt.Values[i].compile(w); err != nil {
						return err
					}
					fmt.Fprintf(w, ")")
					return nil
				}
				fmt.Fprintf(w, "go.map_set(%s, %s,", zigTypeOf(mtype.Key()), zigTypeOf(mtype.Elem()))
				if err := expr.X.compile(w); err != nil {
					return err
				}
				fmt.Fprintf(w, ", ")
				if err := expr.Index.compile(w); err != nil {
					return err
				}
				fmt.Fprintf(w, ", ")
				if err := stmt.Values[i].compile(w); err != nil {
					return err
				}
				fmt.Fprintf(w, ")")
				return nil
			}
			fallthrough
		default:
			if xyz.ValueOf(variable) == Expressions.Identifier {
				ident := Expressions.Identifier.Get(variable)
				if ident.Name.Value == "_" {
					fmt.Fprintf(w, "go.use(")
					if err := stmt.Values[i].compile(w); err != nil {
						return err
					}
					fmt.Fprintf(w, ")")
					break
				}
			}
			if err := variable.compile(w); err != nil {
				return err
			}
			fmt.Fprintf(w, " %s ", stmt.Token.Value)
			switch variable.TypeAndValue().Type.(type) {
			case *types.Interface:
				vtype := stmt.Values[i].TypeAndValue().Type
				if strings.HasPrefix(zigTypeOf(vtype), "?*") {
					fmt.Fprintf(w, "package.interface{.rtype=%s,.value=", zigReflectTypeOf(vtype))
					if err := stmt.Values[i].compile(w); err != nil {
						return nil
					}
					fmt.Fprintf(w, "}")
				} else {
					fmt.Fprintf(w, "package.interface.pack(go, %s, %s, ", zigTypeOf(vtype), zigReflectTypeOf(vtype))
					if err := stmt.Values[i].compile(w); err != nil {
						return err
					}
					fmt.Fprintf(w, ")")
				}
			default:
				if err := stmt.Values[i].compile(w); err != nil {
					return err
				}
			}

		}
	}
	return nil
}
