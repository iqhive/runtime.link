package zigc

import (
	"fmt"
	"go/types"
	"strings"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func (zig Target) FunctionDefinition(decl source.FunctionDefinition) error {
	fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
	body, ok := decl.Body.Get()
	if !ok {
		return decl.Errorf("function missing body")
	}
	for i, stmt := range body.Statements {
		if xyz.ValueOf(stmt) == source.Statements.Defer {
			stmt := source.Statements.Defer.Get(stmt)
			stmt.OutermostScope = true
			body.Statements[i] = source.Statements.Defer.As(stmt)
		}
	}
	if decl.IsTest {
		fmt.Fprintf(zig, "test \"%s\" { var chan = go.routine{}; const goto = &chan; defer goto.exit();", strings.TrimPrefix(decl.Name.String, "Test"))
		t, ok := decl.Type.Arguments.Fields[0].Names.Get()
		if ok {
			fmt.Fprintf(zig, "const %[1]s = go.testing{}; go.use(%[1]s);", zig.toString(t[0]))
		}
		for _, stmt := range body.Statements {
			zig.Tabs++
			if err := zig.Statement(stmt); err != nil {
				return err
			}
			zig.Tabs--
		}
		fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
		fmt.Fprintf(zig, "}")
		fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
		return nil
	}
	receiver, isMethod := decl.Receiver.Get()
	var fnName = decl.Name.String
	if isMethod {
		fnName = fmt.Sprintf(`@"%s.%s"`, receiver.Fields[0].Type.TypeAndValue().Type.(*types.Named).Obj().Name(), fnName)
	}
	if decl.Name.String == "main" {
		fmt.Fprintf(zig, "pub fn main() void { var chan = go.routine{}; const goto = &chan; go.use(goto);")
	} else {
		fmt.Fprintf(zig, "pub fn %s(default: ?*go.routine", fnName)
		if isMethod {
			field := receiver.Fields[0]
			var name = "_"
			names, hasName := field.Names.Get()
			if hasName {
				name = names[0].String
			}
			fmt.Fprintf(zig, ", %s: %s", name, zig.Type(field.Type))
		}
		{
			var i int
			for _, param := range decl.Type.Arguments.Fields {
				names, ok := param.Names.Get()
				if !ok {
					return param.Location.Errorf("missing names for function argument")
				}
				for _, name := range names {
					fmt.Fprintf(zig, ", ")
					fmt.Fprintf(zig, "%s: %s", zig.toString(name), zig.Type(param.Type))
					i++
				}
			}
		}
		fmt.Fprintf(zig, ") ")
		results, ok := decl.Type.Results.Get()
		if ok {
			switch len(results.Fields) {
			case 1:
				fmt.Fprintf(zig, "%s ", zig.Type(results.Fields[0].Type))
			default:
				fmt.Fprintf(zig, ".{")
				for i, field := range results.Fields {
					if i > 0 {
						fmt.Fprintf(zig, ", ")
					}
					fmt.Fprintf(zig, "%s", zig.Type(field.Type))
				}
				fmt.Fprintf(zig, "} ")
			}
		} else {
			fmt.Fprintf(zig, "void ")
		}
		fmt.Fprintf(zig, "{go.use(.{")
		var i int
		if isMethod {
			field := receiver.Fields[0]
			names, hasName := field.Names.Get()
			if hasName {
				fmt.Fprintf(zig, "%s", names[0].String)
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
					fmt.Fprintf(zig, ", ")
				}
				fmt.Fprintf(zig, "%s", name.String)
				i++
			}
		}
		fmt.Fprintf(zig, "});")
		fmt.Fprintf(zig, "var chan = go.routine{}; const goto: *go.routine = if (default) |select| select else &chan; if (default == null) {defer goto.exit();}")
	}
	for _, stmt := range body.Statements {
		zig.Tabs++
		if err := zig.Statement(stmt); err != nil {
			return err
		}
		zig.Tabs--
	}
	fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
	fmt.Fprintf(zig, "}")
	fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
	// Interface wrapper.
	if isMethod {
		named := receiver.Fields[0].Type.TypeAndValue().Type.(*types.Named)
		fmt.Fprintf(zig, `pub fn @"%s.%s.%s.(itfc)"(default: ?*go.routine`, decl.Package, named.Obj().Name(), decl.Name.String)
		field := receiver.Fields[0]
		var name = "_"
		names, hasName := field.Names.Get()
		if hasName {
			name = names[0].String
		}
		fmt.Fprintf(zig, ", %s: *const anyopaque", name)
		{
			var i int
			for _, param := range decl.Type.Arguments.Fields {
				names, ok := param.Names.Get()
				if !ok {
					return param.Location.Errorf("missing names for function argument")
				}
				for _, name := range names {
					fmt.Fprintf(zig, ", ")
					fmt.Fprintf(zig, "%s: %s", zig.toString(name), zig.Type(param.Type))
					i++
				}
			}
		}
		fmt.Fprintf(zig, ") ")
		results, ok := decl.Type.Results.Get()
		if ok {
			switch len(results.Fields) {
			case 1:
				fmt.Fprintf(zig, "%s ", zig.Type(results.Fields[0].Type))
			default:
				return results.Opening.Errorf("unsupported number of function results: %d", len(results.Fields))
			}
		} else {
			fmt.Fprintf(zig, "void ")
		}
		fmt.Fprintf(zig, "{ return %s(default, @as(*const %s, @ptrCast(%s)).*", fnName, zig.Type(field.Type), name)
		{
			var i int
			for _, param := range decl.Type.Arguments.Fields {
				names, ok := param.Names.Get()
				if !ok {
					return param.Location.Errorf("missing names for function argument")
				}
				for _, name := range names {
					fmt.Fprintf(zig, ", ")
					fmt.Fprintf(zig, "%v", name)
					i++
				}
			}
		}
		fmt.Fprintf(zig, "); }")
		fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
	}
	return nil
}
