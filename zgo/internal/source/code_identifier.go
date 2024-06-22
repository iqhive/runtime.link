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

	string

	Shadow int // number of shadowed identifiers

	Mutable bool // mutability analysis result
	Escapes bool // escape analysis result
	Package bool // identifier is global to the package and not defined within a sub-scope.
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
	var global bool
	if object != nil {
		for parent := object.Parent(); parent != nil; parent = parent.Parent() {
			if parent.Lookup(in.Name) != nil {
				shadow++
			}
		}
		parent := object.Parent()
		if parent != nil {
			global = parent.Parent() == types.Universe
		}
	}
	return Identifier{
		typed:    typed{tv: pkg.Types[in]},
		Location: pkg.location(in.Pos()),
		string:   in.Name,
		Shadow:   shadow,
		Package:  global,
		Mutable:  true,
		Escapes:  true,
	}
}

func (id Identifier) String() string {
	return toString(id)
}

func (id Identifier) compile(w io.Writer, tabs int) error {
	if id.string == "_" {
		_, err := w.Write([]byte("_"))
		return err
	}
	if id.Shadow > 0 {
		fmt.Fprintf(w, `@"%s.%d"`, id.string, id.Shadow)
		return nil
	}
	_, err := w.Write([]byte(id.string))
	return err
}
