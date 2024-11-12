package parser

import (
	"go/ast"
	"reflect"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func loadSpecification(pkg *source.Package, node ast.Spec, constant bool, top bool) source.Specification {
	switch spec := node.(type) {
	case *ast.TypeSpec:
		return source.Specifications.Type.New(loadSpecificationType(pkg, spec, top))
	case *ast.ValueSpec:
		return source.Specifications.Value.New(loadSpecificationValue(pkg, spec, constant, top))
	case *ast.ImportSpec:
		return source.Specifications.Import.New(loadImport(pkg, spec))
	default:
		panic("unexpected specification type " + reflect.TypeOf(spec).String())
	}
}

func loadImport(pkg *source.Package, in *ast.ImportSpec) source.SpecificationImport {
	var out source.SpecificationImport
	out.Location = locationRangeIn(pkg, in.Pos(), in.End())
	if in.Doc != nil {
		out.Documentation = xyz.New(loadCommentGroup(pkg, in.Doc))
	}
	if in.Name != nil {
		out.Name = xyz.New(loadIdentifier(pkg, in.Name))
	}
	out.Path = loadConstant(pkg, in.Path)
	if in.Comment != nil {
		out.Comment = xyz.New(loadCommentGroup(pkg, in.Comment))
	}
	out.End = locationIn(pkg, in.End())
	return out
}

func loadSpecificationType(pkg *source.Package, in *ast.TypeSpec, outer bool) source.SpecificationType {
	var out source.SpecificationType
	out.PackageLevelScope = outer
	out.Location = locationRangeIn(pkg, in.Pos(), in.End())
	if in.Doc != nil {
		out.Documentation = xyz.New(loadCommentGroup(pkg, in.Doc))
	}
	out.Name = loadIdentifier(pkg, in.Name)
	if in.TypeParams != nil {
		out.TypeParameters = xyz.New(loadFieldList(pkg, in.TypeParams))
	}
	out.Assign = locationIn(pkg, in.Assign)
	out.Type = loadType(pkg, in.Type)
	out.Typed = typedIn(pkg, in.Type)
	out.Package = pkg.Name
	return out
}

func loadSpecificationValue(pkg *source.Package, in *ast.ValueSpec, constant bool, top bool) source.SpecificationValue {
	var out source.SpecificationValue
	out.Const = constant
	out.PackageLevelScope = top
	out.Location = locationRangeIn(pkg, in.Pos(), in.End())
	if in.Doc != nil {
		out.Documentation = xyz.New(loadCommentGroup(pkg, in.Doc))
	}
	for _, name := range in.Names {
		out.Names = append(out.Names, loadIdentifier(pkg, name))
	}
	if in.Type != nil {
		out.Type = xyz.New(loadType(pkg, in.Type))
	}
	for _, value := range in.Values {
		out.Values = append(out.Values, loadExpression(pkg, value))
	}
	if in.Comment != nil {
		out.Comment = xyz.New(loadCommentGroup(pkg, in.Comment))
	}
	return out
}
