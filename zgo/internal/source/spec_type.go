package source

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"

	"runtime.link/xyz"
)

type SpecificationType struct {
	Location

	Documentation  xyz.Maybe[CommentGroup]
	Name           Identifier
	TypeParameters xyz.Maybe[FieldList]
	Assign         Location
	Type           Type
	Package        string
}

func (pkg *Package) loadSpecificationType(in *ast.TypeSpec) SpecificationType {
	var out SpecificationType
	out.Location = pkg.locations(in.Pos(), in.End())
	if in.Doc != nil {
		out.Documentation = xyz.New(pkg.loadCommentGroup(in.Doc))
	}
	out.Name = pkg.loadIdentifier(in.Name)
	if in.TypeParams != nil {
		out.TypeParameters = xyz.New(pkg.loadFieldList(in.TypeParams))
	}
	out.Assign = pkg.location(in.Assign)
	out.Type = pkg.loadType(in.Type)
	out.Package = pkg.Name
	return out
}

func (spec SpecificationType) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "const @\"%s.%s\" = %s; go.use(@\"%[1]s.%[2]s\"); ", spec.Package, spec.Name.Name.Value, zigTypeOf(spec.Type.TypeAndValue().Type))
	fmt.Fprintf(w, "const @\"%s.%s.(type)\" = go.rtype{", spec.Package, spec.Name.Name.Value)
	fmt.Fprintf(w, ".name=%q,", spec.Name.Name.Value)
	kind := kindOf(spec.Type.TypeAndValue().Type)
	fmt.Fprintf(w, ".kind=go.rkind.%s, ", kind)
	switch rtype := spec.Type.TypeAndValue().Type.(type) {
	case *types.Struct:
		fmt.Fprintf(w, ".data=go.rdata{.%s=&[_]go.field{", kind)
		for i := range rtype.NumFields() {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			field := rtype.Field(i)
			fmt.Fprintf(w, "{.name=%q,.type=%s,.offset=@offsetOf(%s,%[1]s),.exported=%v,.embedded=%v}",
				field.Name(), field.Type().String(), spec.Name.Name.Value, field.Exported(), field.Anonymous())
		}
		fmt.Fprintf(w, "}}")
	default:
		fmt.Fprintf(w, ".data=go.rdata{%s: void}", kind)
	}
	fmt.Fprintf(w, "}; go.use(@\"%s.%s.(type)\")", spec.Package, spec.Name.Name.Value)
	return nil
}

func kindOf(t types.Type) string {
	switch t := t.(type) {
	case *types.Basic:
		switch t.Kind() {
		case types.Bool, types.UntypedBool:
			return "Bool"
		case types.Int, types.UntypedInt:
			return "Int"
		case types.Int8:
			return "Int8"
		case types.Int16:
			return "Int16"
		case types.Int32:
			return "Int32"
		case types.Int64:
			return "Int64"
		case types.Uint:
			return "Uint"
		case types.Uint8:
			return "Uint8"
		case types.Uint16:
			return "Uint16"
		case types.Uint32:
			return "Uint32"
		case types.Uint64:
			return "Uint64"
		case types.Uintptr:
			return "Uintptr"
		case types.Float32:
			return "Float32"
		case types.Float64, types.UntypedFloat:
			return "Float64"
		case types.Complex64:
			return "Complex64"
		case types.Complex128, types.UntypedComplex:
			return "Complex128"
		case types.String:
			return "String"
		case types.UnsafePointer:
			return "UnsafePointer"
		default:
			panic("unexpected kindOf: " + t.String())
		}
	case *types.Array:
		return "Array"
	case *types.Chan:
		return "Chan"
	case *types.Slice:
		return "Slice"
	case *types.Signature:
		return "Func"
	case *types.Interface:
		return "Interface"
	case *types.Map:
		return "Map"
	case *types.Pointer:
		return "Pointer"
	case *types.Struct:
		return "Struct"
	}
	panic("unexpected kindOf: " + t.String())
}
