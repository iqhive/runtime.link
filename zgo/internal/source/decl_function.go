package source

import (
	"fmt"
	"go/ast"
	"io"

	"runtime.link/xyz"
)

type DeclarationFunction struct {
	Documentation xyz.Maybe[CommentGroup]
	Receiver      xyz.Maybe[FieldList]
	Name          Identifier
	Type          TypeFunction
	Body          StatementBlock
}

func (pkg *Package) loadDeclarationFunction(in *ast.FuncDecl) DeclarationFunction {
	var out DeclarationFunction
	if in.Doc != nil {
		out.Documentation = xyz.New(pkg.loadCommentGroup(in.Doc))
	}
	if in.Recv != nil {
		out.Receiver = xyz.New(pkg.loadFieldList(in.Recv))
	}
	out.Name = pkg.loadIdentifier(in.Name)
	out.Type = pkg.loadTypeFunction(in.Type)
	out.Body = pkg.loadStatementBlock(in.Body)
	return out
}

func (decl DeclarationFunction) compile(w io.Writer) error {
	if decl.Name.Name.Value == "main" {
		fmt.Fprintf(w, "pub fn main() void {var chan = runtime.G{}; var go = &chan; defer go.exit();\n")
	} else {
		fmt.Fprintf(w, "pub fn %s(go: *runtime.G", decl.Name.Name.Value)
		for _, param := range decl.Type.Arguments.Fields {
			names, ok := param.Names.Get()
			if !ok {
				return fmt.Errorf("missing names for function argument")
			}
			for _, name := range names {
				fmt.Fprintf(w, ", %s: %s", name.Name.Value, zigTypeOf(param.Type.TypeAndValue().Type))
			}
		}
		fmt.Fprintf(w, ") ")
		results, ok := decl.Type.Results.Get()
		if ok {
			switch len(results.Fields) {
			case 1:
				fmt.Fprintf(w, "%s ", zigTypeOf(results.Fields[0].Type.TypeAndValue().Type))
			default:
				return fmt.Errorf("unsupported number of function results: %d", len(results.Fields))
			}
		} else {
			fmt.Fprintf(w, "void ")
		}
		fmt.Fprintf(w, "{go.use(.{")
		var i int
		for _, param := range decl.Type.Arguments.Fields {
			names, ok := param.Names.Get()
			if !ok {
				return fmt.Errorf("missing names for function argument")
			}
			for _, name := range names {
				if i > 0 {
					fmt.Fprintf(w, ", ")
				}
				fmt.Fprintf(w, "%s", name.Name.Value)
				i++
			}
		}
		fmt.Fprintf(w, "});\n")
	}
	for _, stmt := range decl.Body.Statements {
		if err := stmt.compile(w); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "}\n")
	return nil
}
