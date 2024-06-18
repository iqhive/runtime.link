package source

import (
	"go/ast"
	"io"
	"reflect"

	"runtime.link/xyz"
)

type Specification xyz.Switch[Node, struct {
	Bad xyz.Case[Specification, Bad]

	Type   xyz.Case[Specification, SpecificationType]
	Value  xyz.Case[Specification, SpecificationValue]
	Import xyz.Case[Specification, SpecificationImport]
}]

var Specifications = xyz.AccessorFor(Specification.Values)

func (pkg *Package) loadSpecification(node ast.Spec, constant bool, top bool) Specification {
	switch spec := node.(type) {
	case *ast.TypeSpec:
		return Specifications.Type.New(pkg.loadSpecificationType(spec, top))
	case *ast.ValueSpec:
		return Specifications.Value.New(pkg.loadSpecificationValue(spec, constant, top))
	case *ast.ImportSpec:
		return Specifications.Import.New(pkg.loadImport(spec))
	default:
		panic("unexpected specification type " + reflect.TypeOf(spec).String())
	}
}

func (spec Specification) sources() Location {
	value, _ := spec.Get()
	return value.sources()
}

func (spec Specification) compile(w io.Writer, tabs int) error {
	value, _ := spec.Get()
	return value.compile(w, tabs)
}
