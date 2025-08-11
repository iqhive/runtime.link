package escape

import (
	"go/ast"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func Analysis(pkg source.Package) source.Package {
	var escape = make(graph)
	for i := range pkg.Files {
		escape.RouteFile(&pkg.Files[i])
	}
	return pkg
}

type graph map[ast.Node]information

func (escape graph) Make(node ast.Node) func() bool {
	return func() bool {
		info, ok := escape[node]
		if !ok {
			return false
		}
		if info.escapes {
			return true
		}
		var seen = make(map[ast.Node]struct{})
		for _, buddy := range info.buddies {
			if escape.view(seen, &buddy) {
				info.escapes = true
				escape[node] = info
				return true
			}
		}
		return false
	}
}

func (escape graph) view(seen map[ast.Node]struct{}, node *ast.Node) bool {
	if _, ok := seen[*node]; ok {
		return false
	}
	seen[*node] = struct{}{}
	info, ok := escape[*node]
	if !ok {
		return false
	}
	if info.escapes {
		return true
	}
	for _, buddy := range info.buddies {
		if escape.view(seen, &buddy) {
			info.escapes = true
			escape[*node] = info
			return true
		}
	}
	return false
}

type information struct {
	escapes bool
	buddies []ast.Node
}

func (escape graph) RouteFile(file *source.File) {
	for i := range file.Definitions {
		escape.RouteDefinition(&file.Definitions[i])
	}
}

func (escape graph) RouteDefinition(def *source.Definition) {
	switch xyz.ValueOf(*def) {
	case source.Definitions.Function:
		*def = source.Definitions.Function.New(escape.RouteFunction(source.Definitions.Function.Get(*def)))
	case source.Definitions.Variable:
		*def = source.Definitions.Variable.New(escape.RouteVariable(source.Definitions.Variable.Get(*def)))
	}
}

func (escape graph) RouteFunction(def source.FunctionDefinition) source.FunctionDefinition {
	return def
}

func (escape graph) RouteVariable(def source.VariableDefinition) source.VariableDefinition {
	var escapes bool
	var buddies []ast.Node
	if def.Global {
		escapes = true
	}
	if value, ok := def.Value.Get(); ok {
		node, _ := value.Get()
		buddies = append(buddies, source.LocationOf(node).Node)
		def.Value = xyz.New(escape.RouteExpression(value))
	}
	if escapes {
		def.Name.Escapes = escape.Make(def.Name.Node)
		escape[def.Name.Node] = information{
			escapes: true,
		}
	}
	return def
}

func (escape graph) RouteExpression(expr source.Expression) source.Expression {
	return expr
}
