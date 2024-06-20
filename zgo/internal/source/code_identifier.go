package source

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"
)

type Identifier struct {
	typed

	Location

	Name WithLocation[string]

	Shadow int

	Variable bool
}

func (pkg *Package) loadIdentifier(in *ast.Ident) Identifier {
	var shadow int = -1
	var object types.Object
	if obj := pkg.Uses[in]; obj != nil {
		object = obj
	}
	if obj := pkg.Defs[in]; obj != nil {
		object = obj
	}
	if object != nil {
		for parent := object.Parent(); parent != nil; parent = parent.Parent() {
			if parent.Lookup(in.Name) != nil {
				shadow++
			}
		}
	}
	var variable bool
	switch object.(type) {
	case *types.Var:
		variable = true
	}
	return Identifier{
		Location: pkg.locations(in.Pos(), in.End()),
		typed:    typed{tv: pkg.Types[in]},
		Name: WithLocation[string]{
			Value:          in.Name,
			SourceLocation: pkg.location(in.NamePos),
		},
		Shadow:   shadow,
		Variable: variable,
	}
}

func (id Identifier) compile(w io.Writer, tabs int) error {
	if id.Shadow > 0 {
		fmt.Fprintf(w, `@"%s.%d"`, id.Name.Value, id.Shadow)
		return nil
	}
	_, err := w.Write([]byte(id.Name.Value))
	return err
}
