package source

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"reflect"
	"strings"

	"runtime.link/xyz"
)

type Type xyz.Switch[TypedNode, struct {
	Bad xyz.Case[Type, Bad]

	Identifier    xyz.Case[Type, Identifier]
	Parenthesized xyz.Case[Type, Parenthesized]
	Selection     xyz.Case[Type, Selection]
	Star          xyz.Case[Type, Star]
	TypeArray     xyz.Case[Type, TypeArray]
	TypeChannel   xyz.Case[Type, TypeChannel]
	TypeFunction  xyz.Case[Type, TypeFunction]
	TypeInterface xyz.Case[Type, TypeInterface]
	TypeMap       xyz.Case[Type, TypeMap]
	TypeStruct    xyz.Case[Type, TypeStruct]
	TypeVariadic  xyz.Case[Type, TypeVariadic]
}]

var Types = xyz.AccessorFor(Type.Values)

func (e Type) sources() Location {
	value, _ := e.Get()
	return value.sources()
}

func (e Type) TypeAndValue() types.TypeAndValue {
	value, _ := e.Get()
	return value.TypeAndValue()
}

func (e Type) compile(w io.Writer, tabs int) error {
	value, _ := e.Get()
	return value.compile(w, tabs)
}

func (pkg *Package) loadType(node ast.Node) Type {
	switch typ := node.(type) {
	case *ast.BadExpr:
		return Types.Bad.New(pkg.loadBad(typ, typ.From, typ.To))
	case *ast.Ident:
		return Types.Identifier.New(pkg.loadIdentifier(typ))
	case *ast.ParenExpr:
		return Types.Parenthesized.New(pkg.loadParenthesized(typ))
	case *ast.SelectorExpr:
		return Types.Selection.New(pkg.loadSelection(typ))
	case *ast.StarExpr:
		return Types.Star.New(pkg.loadStar(typ))
	case *ast.ArrayType:
		return Types.TypeArray.New(pkg.loadTypeArray(typ))
	case *ast.ChanType:
		return Types.TypeChannel.New(pkg.loadTypeChannel(typ))
	case *ast.FuncType:
		return Types.TypeFunction.New(pkg.loadTypeFunction(typ))
	case *ast.InterfaceType:
		return Types.TypeInterface.New(pkg.loadTypeInterface(typ))
	case *ast.MapType:
		return Types.TypeMap.New(pkg.loadTypeMap(typ))
	case *ast.StructType:
		return Types.TypeStruct.New(pkg.loadTypeStruct(typ))
	case *ast.Ellipsis:
		return Types.TypeVariadic.New(pkg.loadVariadic(typ))
	default:
		panic("unexpected type " + reflect.TypeOf(typ).String())
	}
}

type TypeArray struct {
	typed

	Location

	OpenBracket Location
	Length      xyz.Maybe[Expression]
	ElementType Type
}

func (pkg *Package) loadTypeArray(in *ast.ArrayType) TypeArray {
	var length xyz.Maybe[Expression]
	if in.Len != nil {
		length = xyz.New(pkg.loadExpression(in.Len))
	}
	return TypeArray{
		Location:    pkg.locations(in.Pos(), in.End()),
		typed:       typed{pkg.Types[in]},
		OpenBracket: pkg.location(in.Lbrack),
		Length:      length,
		ElementType: pkg.loadType(in.Elt),
	}
}

func (e TypeArray) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "%s", zigTypeOf(e.TypeAndValue().Type))
	return nil
}

type TypeChannel struct {
	typed

	Location

	Begin Location
	Arrow Location
	Dir   ast.ChanDir
	Value Expression
}

func (pkg *Package) loadTypeChannel(in *ast.ChanType) TypeChannel {
	return TypeChannel{
		typed:    typed{pkg.Types[in]},
		Location: pkg.locations(in.Pos(), in.End()),
		Begin:    pkg.location(in.Begin),
		Arrow:    pkg.location(in.Arrow),
		Dir:      in.Dir,
		Value:    pkg.loadExpression(in.Value),
	}
}

type TypeFunction struct {
	typed

	Location

	Keyword    Location
	TypeParams xyz.Maybe[FieldList]
	Arguments  FieldList
	Results    xyz.Maybe[FieldList]
}

func (pkg *Package) loadTypeFunction(in *ast.FuncType) TypeFunction {
	var results xyz.Maybe[FieldList]
	if in.Results != nil {
		results = xyz.New(pkg.loadFieldList(in.Results))
	}
	var typeparams xyz.Maybe[FieldList]
	if in.TypeParams != nil {
		typeparams = xyz.New(pkg.loadFieldList(in.TypeParams))
	}
	return TypeFunction{
		typed:      typed{pkg.Types[in]},
		Location:   pkg.locations(in.Pos(), in.End()),
		Keyword:    pkg.location(in.Func),
		TypeParams: typeparams,
		Arguments:  pkg.loadFieldList(in.Params),
		Results:    results,
	}
}

func (e TypeFunction) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "%s", zigTypeOf(e.TypeAndValue().Type))
	return nil
}

type TypeInterface struct {
	typed

	Location

	Keyword    Location
	Methods    FieldList
	Incomplete bool
}

func (pkg *Package) loadTypeInterface(in *ast.InterfaceType) TypeInterface {
	return TypeInterface{
		typed:      typed{pkg.Types[in]},
		Location:   pkg.locations(in.Pos(), in.End()),
		Keyword:    pkg.location(in.Interface),
		Methods:    pkg.loadFieldList(in.Methods),
		Incomplete: in.Incomplete,
	}
}

func (e TypeInterface) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "%s", zigTypeOf(e.TypeAndValue().Type))
	return nil
}

type TypeMap struct {
	typed

	Location

	Keyword Location
	Key     Expression
	Value   Expression
}

func (pkg *Package) loadTypeMap(in *ast.MapType) TypeMap {
	return TypeMap{
		typed: typed{pkg.Types[in]},

		Location: pkg.locations(in.Pos(), in.End()),
		Keyword:  pkg.location(in.Map),
		Key:      pkg.loadExpression(in.Key),
		Value:    pkg.loadExpression(in.Value),
	}
}

func (e TypeMap) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "%s", zigTypeOf(e.TypeAndValue().Type))
	return nil
}

type TypeStruct struct {
	typed

	Location

	Keyword    Location
	Fields     FieldList
	Incomplete bool
}

func (pkg *Package) loadTypeStruct(in *ast.StructType) TypeStruct {
	return TypeStruct{
		Location:   pkg.locations(in.Pos(), in.End()),
		typed:      typed{pkg.Types[in]},
		Keyword:    pkg.location(in.Struct),
		Fields:     pkg.loadFieldList(in.Fields),
		Incomplete: in.Incomplete,
	}
}

func (e TypeStruct) compile(w io.Writer, tabs int) error {
	fmt.Fprintf(w, "%s", zigTypeOf(e.TypeAndValue().Type))
	return nil
}

type TypeVariadic struct {
	typed

	Location

	ElementType WithLocation[Type]
}

func (pkg *Package) loadVariadic(in *ast.Ellipsis) TypeVariadic {
	return TypeVariadic{
		typed:    typed{pkg.Types[in]},
		Location: pkg.locations(in.Pos(), in.End()),
		ElementType: WithLocation[Type]{
			Value:          pkg.loadType(in.Elt),
			SourceLocation: pkg.location(in.Ellipsis),
		},
	}
}

func zigTypeOf(t types.Type) string {
	switch typ := t.(type) {
	case *types.Basic:
		switch typ.Kind() {
		case types.Bool:
			return "bool"
		case types.Int, types.UntypedInt:
			return "go.int"
		case types.Int8:
			return "go.int8"
		case types.Int16:
			return "go.int16"
		case types.Int32:
			return "go.int32"
		case types.Int64:
			return "go.int64"
		case types.Uint:
			return "go.uint"
		case types.Uint8:
			return "go.uint8"
		case types.Uint16:
			return "go.uint16"
		case types.Uint32:
			return "go.uint32"
		case types.Uint64:
			return "go.uint64"
		case types.Uintptr:
			return "go.uintptr"
		case types.Float32:
			return "go.float32"
		case types.Float64:
			return "go.float64"
		case types.String:
			return "go.string"
		case types.Complex64:
			return "go.complex64"
		case types.Complex128:
			return "go.complex128"
		default:
			panic("unsupported basic type " + typ.String())
		}
	case *types.Array:
		return fmt.Sprintf("[%d]%s", typ.Len(), zigTypeOf(typ.Elem()))
	case *types.Signature:
		var builder strings.Builder
		builder.WriteString("go.func(go.types(.{")
		for i := 0; i < typ.Params().Len(); i++ {
			param := typ.Params().At(i)
			if i > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(zigTypeOf(param.Type()))
		}
		builder.WriteString("}),go.types(.{")
		if typ.Results().Len() == 0 {
			builder.WriteString("void")
		} else if typ.Results().Len() == 1 {
			builder.WriteString(zigTypeOf(typ.Results().At(0).Type()))
		} else {
			panic("unsupported function type with multiple results")
		}
		builder.WriteString("}))")
		return builder.String()
	case *types.Named:
		return "@\"" + typ.Obj().Pkg().Name() + "." + typ.Obj().Name() + "\""
	case *types.Pointer:
		return "go.pointer(" + zigTypeOf(typ.Elem()) + ")"
	case *types.Slice:
		return "go.slice(" + zigTypeOf(typ.Elem()) + ")"
	case *types.Map:
		if typ.Key().String() == "string" {
			return "go.smap(" + zigTypeOf(typ.Elem()) + ")"
		}
		return "go.map(" + zigTypeOf(typ.Key()) + ", " + zigTypeOf(typ.Elem()) + ")"
	case *types.Interface:
		if typ.Empty() {
			return "go.interface"
		}
		panic("unsupported type " + typ.String())
	case *types.Struct:
		var builder strings.Builder
		builder.WriteString("struct {")
		for i := 0; i < typ.NumFields(); i++ {
			if i > 0 {
				builder.WriteString(", ")
			}
			field := typ.Field(i)
			builder.WriteString(field.Name())
			builder.WriteString(": ")
			builder.WriteString(zigTypeOf(field.Type()))
		}
		builder.WriteString("}")
		return builder.String()
	default:
		panic("unsupported type " + reflect.TypeOf(typ).String())
	}
}

func zigReflectTypeOf(t types.Type) string {
	switch typ := t.(type) {
	case *types.Basic:
		switch typ.Kind() {
		case types.Bool:
			return "&go.@\"bool.(type)\""
		case types.Int, types.UntypedInt:
			return "&go.@\"int.(type)\""
		case types.Int8:
			return "&go.@\"int8.(type)\""
		case types.Int16:
			return "&go.@\"int16.(type)\""
		case types.Int32:
			return "&go.@\"int32.(type)\""
		case types.Int64:
			return "&go.@\"int64.(type)\""
		case types.Uint:
			return "&go.@\"uint.(type)\""
		case types.Uint8:
			return "&go.@\"uint8.(type)\""
		case types.Uint16:
			return "&go.@\"uint16.(type)\""
		case types.Uint32:
			return "&go.@\"uint32.(type)\""
		case types.Uint64:
			return "&go.@\"uint64.(type)\""
		case types.Uintptr:
			return "&go.@\"uintptr.(type)\""
		case types.Float32:
			return "&go.@\"float32.(type)\""
		case types.Float64:
			return "&go.@\"float64.(type)\""
		case types.String:
			return "&go.@\"string)\""
		case types.Complex64:
			return "&go.@\"complex64)\""
		case types.Complex128:
			return "&go.@\"complex128)\""
		default:
			panic("unsupported basic type " + typ.String())
		}
	case *types.Named:
		return "&@\"" + typ.Obj().Pkg().Name() + "." + typ.Obj().Name() + ".(type)\""
	case *types.Pointer:
		return "go.rptr(" + zigReflectTypeOf(typ.Elem()) + ")"
	default:
		panic("unsupported type " + reflect.TypeOf(typ).String())
	}
}
