package source

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"strings"

	"runtime.link/xyz"
)

type DeclarationFunction struct {
	Location

	Documentation xyz.Maybe[CommentGroup]
	Receiver      xyz.Maybe[FieldList]

	Package string

	Name Identifier
	Type TypeFunction
	Body xyz.Maybe[StatementBlock]

	// Test is true when the function is a test function, within a test package.
	Test bool
}

func (pkg *Package) loadDeclarationFunction(in *ast.FuncDecl) DeclarationFunction {
	var out DeclarationFunction
	out.Location = pkg.locations(in.Pos(), in.End())
	if in.Doc != nil {
		out.Documentation = xyz.New(pkg.loadCommentGroup(in.Doc))
	}
	if in.Recv != nil {
		out.Receiver = xyz.New(pkg.loadFieldList(in.Recv))
	}
	out.Name = pkg.loadIdentifier(in.Name)
	out.Type = pkg.loadTypeFunction(in.Type)
	if in.Body != nil {
		out.Body = xyz.New(pkg.loadStatementBlock(in.Body))
	}
	out.Package = pkg.Name
	if pkg.Test &&
		strings.HasPrefix(out.Name.String(), "Test") &&
		len(out.Type.Arguments.Fields) == 1 &&
		out.Type.Arguments.Fields[0].Type.TypeAndValue().Type.String() == "*testing.T" {
		out.Test = true
	}
	return out
}

func (decl DeclarationFunction) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
	body, ok := decl.Body.Get()
	if !ok {
		return decl.Errorf("function missing body")
	}
	for i, stmt := range body.Statements {
		if xyz.ValueOf(stmt) == Statements.Defer {
			stmt := Statements.Defer.Get(stmt)
			stmt.OutermostScope = true
			body.Statements[i] = Statements.Defer.As(stmt)
		}
	}
	if decl.Test {
		fmt.Fprintf(w, "test \"%s\" { var chan = go.routine{}; const goto = &chan; defer goto.exit();", strings.TrimPrefix(decl.Name.String(), "Test"))
		t, ok := decl.Type.Arguments.Fields[0].Names.Get()
		if ok {
			fmt.Fprintf(w, "const %[1]s = go.testing{}; go.use(%[1]s);", toString(t[0]))
		}
		for _, stmt := range body.Statements {
			if err := stmt.compile(w, tabs+1); err != nil {
				return err
			}
		}
		fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
		fmt.Fprintf(w, "}")
		fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
		return nil
	}
	receiver, isMethod := decl.Receiver.Get()
	var fnName = decl.Name.String()
	if isMethod {
		fnName = fmt.Sprintf(`@"%s.%s"`, receiver.Fields[0].Type.TypeAndValue().Type.(*types.Named).Obj().Name(), fnName)
	}
	if decl.Name.String() == "main" {
		fmt.Fprintf(w, "pub fn main() void { var chan = go.routine{}; const goto = &chan; go.use(goto);")
	} else {
		fmt.Fprintf(w, "pub fn %s(default: ?*go.routine", fnName)
		if isMethod {
			field := receiver.Fields[0]
			var name = "_"
			names, hasName := field.Names.Get()
			if hasName {
				name = names[0].String()
			}
			fmt.Fprintf(w, ", %s: %s", name, field.Type.ZigType())
		}
		{
			var i int
			for _, param := range decl.Type.Arguments.Fields {
				names, ok := param.Names.Get()
				if !ok {
					return param.Location.Errorf("missing names for function argument")
				}
				for _, name := range names {
					fmt.Fprintf(w, ", ")
					fmt.Fprintf(w, "%s: %s", toString(name), param.Type.ZigType())
					i++
				}
			}
		}
		fmt.Fprintf(w, ") ")
		results, ok := decl.Type.Results.Get()
		if ok {
			switch len(results.Fields) {
			case 1:
				fmt.Fprintf(w, "%s ", results.Fields[0].Type.ZigType())
			default:
				fmt.Fprintf(w, ".{")
				for i, field := range results.Fields {
					if i > 0 {
						fmt.Fprintf(w, ", ")
					}
					fmt.Fprintf(w, "%s", field.Type.ZigType())
				}
				fmt.Fprintf(w, "} ")
			}
		} else {
			fmt.Fprintf(w, "void ")
		}
		fmt.Fprintf(w, "{go.use(.{")
		var i int
		if isMethod {
			field := receiver.Fields[0]
			names, hasName := field.Names.Get()
			if hasName {
				fmt.Fprintf(w, "%s", names[0])
				i++
			}
		}
		for _, param := range decl.Type.Arguments.Fields {
			names, ok := param.Names.Get()
			if !ok {
				return param.Location.Errorf("missing names for function argument")
			}
			for _, name := range names {
				if i > 0 {
					fmt.Fprintf(w, ", ")
				}
				fmt.Fprintf(w, "%s", name)
				i++
			}
		}
		fmt.Fprintf(w, "});")
		fmt.Fprintf(w, "var chan = go.routine{}; const goto: *go.routine = if (default) |select| select else &chan; if (default == null) {defer goto.exit();}")
	}
	for _, stmt := range body.Statements {
		if err := stmt.compile(w, tabs+1); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
	fmt.Fprintf(w, "}")
	fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
	// Interface wrapper.
	if isMethod {
		named := receiver.Fields[0].Type.TypeAndValue().Type.(*types.Named)
		fmt.Fprintf(w, `pub fn @"%s.%s.%s.(itfc)"(default: ?*go.routine`, decl.Package, named.Obj().Name(), decl.Name.String())
		field := receiver.Fields[0]
		var name = "_"
		names, hasName := field.Names.Get()
		if hasName {
			name = names[0].String()
		}
		fmt.Fprintf(w, ", %s: *const anyopaque", name)
		{
			var i int
			for _, param := range decl.Type.Arguments.Fields {
				names, ok := param.Names.Get()
				if !ok {
					return param.Location.Errorf("missing names for function argument")
				}
				for _, name := range names {
					fmt.Fprintf(w, ", ")
					fmt.Fprintf(w, "%s: %s", toString(name), param.Type.ZigType())
					i++
				}
			}
		}
		fmt.Fprintf(w, ") ")
		results, ok := decl.Type.Results.Get()
		if ok {
			switch len(results.Fields) {
			case 1:
				fmt.Fprintf(w, "%s ", results.Fields[0].Type.ZigType())
			default:
				return results.Opening.Errorf("unsupported number of function results: %d", len(results.Fields))
			}
		} else {
			fmt.Fprintf(w, "void ")
		}
		fmt.Fprintf(w, "{ return %s(default, @as(*const %s, @ptrCast(%s)).*", fnName, field.Type.ZigType(), name)
		{
			var i int
			for _, param := range decl.Type.Arguments.Fields {
				names, ok := param.Names.Get()
				if !ok {
					return param.Location.Errorf("missing names for function argument")
				}
				for _, name := range names {
					fmt.Fprintf(w, ", ")
					fmt.Fprintf(w, "%s", name)
					i++
				}
			}
		}
		fmt.Fprintf(w, "); }")
		fmt.Fprintf(w, "\n%s", strings.Repeat("\t", tabs))
	}
	return nil
}
