package parser

import (
	"go/ast"
	"go/token"
	"reflect"
	"strings"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func loadDefinitions(pkg *source.Package, node ast.Decl, global bool) []source.Definition {
	switch decl := node.(type) {
	case *ast.BadDecl:
		return []source.Definition{source.Definitions.Invalid.New(loadBad(pkg, decl, decl.From, decl.To))}
	case *ast.FuncDecl:
		return []source.Definition{source.Definitions.Function.New(loadDeclarationFunction(pkg, decl))}
	case *ast.GenDecl:
		var defs []source.Definition
		for _, spec := range decl.Specs {
			switch spec := spec.(type) {
			case *ast.TypeSpec:
				defs = append(defs, source.Definitions.Type.New(loadDefinitionType(pkg, spec, global)))
			case *ast.ValueSpec:
				if decl.Tok == token.CONST {
					for i, name := range spec.Names {
						defs = append(defs, source.Definitions.Constant.New(source.ConstantDefinition{
							Location: locationIn(pkg, spec, spec.Pos()),
							Name:     source.DefinedConstant(loadIdentifier(pkg, name)),
							Typed:    typedIn(pkg, spec.Values[i]),
							Global:   global,
							Value:    loadExpression(pkg, spec.Values[i]),
						}))
					}
				} else {
					for i, name := range spec.Names {
						var value xyz.Maybe[source.Expression]
						var typed source.Typed
						if len(spec.Values) > 0 {
							value = xyz.New(loadExpression(pkg, spec.Values[i]))
							typed = typedIn(pkg, spec.Values[i])
						}
						var vtype xyz.Maybe[source.Type]
						if spec.Type != nil {
							vtype = xyz.New(loadType(pkg, spec.Type))

						}
						defs = append(defs, source.Definitions.Variable.New(source.VariableDefinition{
							Location: locationIn(pkg, name, name.Pos()),
							Name:     source.DefinedVariable(loadIdentifier(pkg, name)),
							Global:   global,
							Typed:    typed,
							Type:     vtype,
							Value:    value,
						}))
					}
				}
			case *ast.ImportSpec:
			default:
				panic("unexpected specification type " + reflect.TypeOf(spec).String())
			}
		}
		return defs
	default:
		panic("unexpected declaration type " + reflect.TypeOf(decl).String())
	}
}

func loadDeclarationFunction(pkg *source.Package, in *ast.FuncDecl) source.FunctionDefinition {
	var out source.FunctionDefinition
	out.Location = locationRangeIn(pkg, in, in.Pos(), in.End())
	if in.Doc != nil {
		out.Documentation = xyz.New(loadCommentGroup(pkg, in.Doc))
	}
	if in.Recv != nil {
		out.Receiver = xyz.New(loadFieldList(pkg, in.Recv))
	}
	out.Name = source.DefinedFunction(loadIdentifier(pkg, in.Name))
	out.Type = loadTypeFunction(pkg, in.Type)
	if in.Body != nil {
		out.Body = xyz.New(loadStatementBlock(pkg, in.Body))
	}
	out.Package = pkg.Name
	if pkg.Test &&
		strings.HasPrefix(out.Name.String, "Test") &&
		len(out.Type.Arguments.Fields) == 1 &&
		out.Type.Arguments.Fields[0].Type.TypeAndValue().Type.String() == "*testing.T" {
		out.IsTest = true
	}
	return out
}

func loadDefinitionType(pkg *source.Package, in *ast.TypeSpec, outer bool) source.TypeDefinition {
	var out source.TypeDefinition
	out.Global = outer
	out.Location = locationRangeIn(pkg, in, in.Pos(), in.End())
	out.Name = source.DefinedType(loadIdentifier(pkg, in.Name))
	if in.TypeParams != nil {
		out.TypeParameters = xyz.New(loadFieldList(pkg, in.TypeParams))
	}
	out.Type = loadType(pkg, in.Type)
	out.Typed = typedIn(pkg, in.Type)
	out.Package = pkg.Name
	return out
}
