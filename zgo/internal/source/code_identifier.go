package source

import (
	"go/ast"
	"io"
)

type Identifier struct {
	typed

	Location

	Name WithLocation[string]
}

func (pkg *Package) loadIdentifier(in *ast.Ident) Identifier {
	return Identifier{
		Location: pkg.locations(in.Pos(), in.End()),
		typed:    typed{tv: pkg.Types[in]},
		Name: WithLocation[string]{
			Value:          in.Name,
			SourceLocation: pkg.location(in.NamePos),
		},
	}
}

func (id Identifier) compile(w io.Writer, tabs int) error {
	_, err := w.Write([]byte(id.Name.Value))
	return err
}
