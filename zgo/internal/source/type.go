package source

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"reflect"

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

func (e Type) TypeAndValue() types.TypeAndValue {
	value, _ := e.Get()
	return value.TypeAndValue()
}

func (e Type) compile(w io.Writer) error {
	value, _ := e.Get()
	return value.compile(w)
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
		typed:       typed{pkg.Types[in]},
		OpenBracket: Location(in.Lbrack),
		Length:      length,
		ElementType: pkg.loadType(in.Elt),
	}
}

func (e TypeArray) compile(w io.Writer) error {
	fmt.Fprintf(w, "%s", zigTypeOf(e.TypeAndValue().Type))
	return nil
}

type TypeChannel struct {
	ast.Node

	Begin Location
	Arrow Location
	Dir   ast.ChanDir
	Value Expression
}

func (pkg *Package) loadTypeChannel(in *ast.ChanType) TypeChannel {
	return TypeChannel{
		Node:  in,
		Begin: Location(in.Begin),
		Arrow: Location(in.Arrow),
		Dir:   in.Dir,
		Value: pkg.loadExpression(in.Value),
	}
}

type TypeFunction struct {
	ast.Node

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
	return TypeFunction{
		Node:       in,
		Keyword:    Location(in.Func),
		TypeParams: xyz.New(pkg.loadFieldList(in.TypeParams)),
		Arguments:  pkg.loadFieldList(in.Params),
		Results:    results,
	}
}

type TypeInterface struct {
	ast.Node

	Keyword    Location
	Methods    FieldList
	Incomplete bool
}

func (pkg *Package) loadTypeInterface(in *ast.InterfaceType) TypeInterface {
	return TypeInterface{
		Node:       in,
		Keyword:    Location(in.Interface),
		Methods:    pkg.loadFieldList(in.Methods),
		Incomplete: in.Incomplete,
	}
}

type TypeMap struct {
	typed

	Keyword Location
	Key     Expression
	Value   Expression
}

func (pkg *Package) loadTypeMap(in *ast.MapType) TypeMap {
	return TypeMap{
		typed:   typed{pkg.Types[in]},
		Keyword: Location(in.Map),
		Key:     pkg.loadExpression(in.Key),
		Value:   pkg.loadExpression(in.Value),
	}
}

func (e TypeMap) compile(w io.Writer) error {
	fmt.Fprintf(w, "%s", zigTypeOf(e.TypeAndValue().Type))
	return nil
}

type TypeStruct struct {
	ast.Node

	Keyword    Location
	Fields     FieldList
	Incomplete bool
}

func (pkg *Package) loadTypeStruct(in *ast.StructType) TypeStruct {
	return TypeStruct{
		Node:       in,
		Keyword:    Location(in.Struct),
		Fields:     pkg.loadFieldList(in.Fields),
		Incomplete: in.Incomplete,
	}
}

type TypeVariadic struct {
	ast.Node

	ElementType WithLocation[Type]
}

func (pkg *Package) loadVariadic(in *ast.Ellipsis) TypeVariadic {
	return TypeVariadic{
		Node: in,
		ElementType: WithLocation[Type]{
			Value:          pkg.loadType(in.Elt),
			SourceLocation: Location(in.Ellipsis),
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
			return "isize"
		case types.Int8:
			return "i8"
		case types.Int16:
			return "i16"
		case types.Int32:
			return "i32"
		case types.Int64:
			return "i64"
		case types.Uint:
			return "usize"
		case types.Uint8:
			return "u8"
		case types.Uint16:
			return "u16"
		case types.Uint32:
			return "u32"
		case types.Uint64:
			return "u64"
		case types.Uintptr:
			return "usize"
		case types.Float32:
			return "f32"
		case types.Float64:
			return "f64"
		case types.String:
			return "[]const u8"
		case types.Complex64:
			return "[2]f32"
		case types.Complex128:
			return "[2]f64"
		default:
			panic("unsupported basic type " + typ.String())
		}
	case *types.Named:
		return typ.Obj().Pkg().Name() + "." + typ.Obj().Name()
	case *types.Pointer:
		return "*" + zigTypeOf(typ.Elem())
	case *types.Slice:
		return "runtime.slice(" + zigTypeOf(typ.Elem()) + ")"
	case *types.Map:
		if typ.Key().String() == "string" {
			return "runtime.smap(" + zigTypeOf(typ.Elem()) + ")"
		}
		return "runtime.map(" + zigTypeOf(typ.Key()) + ", " + zigTypeOf(typ.Elem()) + ")"
	default:
		panic("unsupported type " + reflect.TypeOf(typ).String())
	}
}
