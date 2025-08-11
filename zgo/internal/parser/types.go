package parser

import (
	"go/ast"
	"reflect"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func loadType(pkg *source.Package, node ast.Node) source.Type {
	switch typ := node.(type) {
	case *ast.BadExpr:
		return source.Types.Bad.New(loadBad(pkg, typ, typ.From, typ.To))
	case *ast.Ident:
		return source.Types.TypeNamed.New(source.DefinedType(loadIdentifier(pkg, typ)))
	case *ast.ParenExpr:
		return source.Types.Parenthesized.New(loadParenthesized(pkg, typ))
	case *ast.SelectorExpr:
		return source.Types.Selection.New(loadSelection(pkg, typ))
	case *ast.StarExpr:
		return source.Types.Pointer.New(loadTypePointer(pkg, typ))
	case *ast.ArrayType:
		return source.Types.TypeArray.New(loadTypeArray(pkg, typ))
	case *ast.ChanType:
		return source.Types.TypeChannel.New(loadTypeChannel(pkg, typ))
	case *ast.FuncType:
		return source.Types.TypeFunction.New(loadTypeFunction(pkg, typ))
	case *ast.InterfaceType:
		return source.Types.TypeInterface.New(loadTypeInterface(pkg, typ))
	case *ast.MapType:
		return source.Types.TypeMap.New(loadTypeMap(pkg, typ))
	case *ast.StructType:
		return source.Types.TypeStruct.New(loadTypeStruct(pkg, typ))
	case *ast.Ellipsis:
		return source.Types.TypeVariadic.New(loadVariadic(pkg, typ))
	case *ast.BinaryExpr:
		return source.Types.Unknown.New(loadTypeUnknown(pkg, typ))
	case *ast.UnaryExpr:
		return source.Types.Unknown.New(loadTypeUnknown(pkg, typ))
	case *ast.IndexExpr:
		return source.Types.Unknown.New(loadTypeUnknown(pkg, typ))
	default:
		panic("unexpected type " + reflect.TypeOf(node).String())
	}
}

func loadTypeUnknown(pkg *source.Package, in ast.Expr) source.TypeUnknown {
	return source.TypeUnknown{
		Location: locationRangeIn(pkg, in, in.Pos(), in.End()),
		Typed:    typedIn(pkg, in),
	}
}

func loadTypeArray(pkg *source.Package, in *ast.ArrayType) source.TypeArray {
	var length xyz.Maybe[source.Expression]
	if in.Len != nil {
		length = xyz.New(loadExpression(pkg, in.Len))
	}
	return source.TypeArray{
		Location:    locationRangeIn(pkg, in, in.Pos(), in.End()),
		Typed:       typedIn(pkg, in),
		OpenBracket: locationIn(pkg, in, in.Lbrack),
		Length:      length,
		ElementType: loadType(pkg, in.Elt),
	}
}

func loadTypeChannel(pkg *source.Package, in *ast.ChanType) source.TypeChannel {
	return source.TypeChannel{
		Typed:    typedIn(pkg, in),
		Location: locationRangeIn(pkg, in, in.Pos(), in.End()),
		Begin:    locationIn(pkg, in, in.Begin),
		Arrow:    locationIn(pkg, in, in.Arrow),
		Dir:      in.Dir,
		Value:    loadExpression(pkg, in.Value),
	}
}

func loadTypePointer(pkg *source.Package, in *ast.StarExpr) source.TypePointer {
	return source.TypePointer(loadStar(pkg, in))
}

func loadTypeFunction(pkg *source.Package, in *ast.FuncType) source.TypeFunction {
	var results xyz.Maybe[source.FieldList]
	if in.Results != nil {
		results = xyz.New(loadFieldList(pkg, in.Results))
	}
	var typeparams xyz.Maybe[source.FieldList]
	if in.TypeParams != nil {
		typeparams = xyz.New(loadFieldList(pkg, in.TypeParams))
	}
	return source.TypeFunction{
		Typed:      typedIn(pkg, in),
		Location:   locationRangeIn(pkg, in, in.Pos(), in.End()),
		Keyword:    locationIn(pkg, in, in.Func),
		TypeParams: typeparams,
		Arguments:  loadFieldList(pkg, in.Params),
		Results:    results,
	}
}

func loadTypeInterface(pkg *source.Package, in *ast.InterfaceType) source.TypeInterface {
	return source.TypeInterface{
		Typed:      typedIn(pkg, in),
		Location:   locationRangeIn(pkg, in, in.Pos(), in.End()),
		Keyword:    locationIn(pkg, in, in.Interface),
		Methods:    loadFieldList(pkg, in.Methods),
		Incomplete: in.Incomplete,
	}
}

func loadTypeMap(pkg *source.Package, in *ast.MapType) source.TypeMap {
	return source.TypeMap{
		Typed: typedIn(pkg, in),

		Location: locationRangeIn(pkg, in, in.Pos(), in.End()),
		Keyword:  locationIn(pkg, in, in.Map),
		Key:      loadExpression(pkg, in.Key),
		Value:    loadExpression(pkg, in.Value),
	}
}

func loadTypeStruct(pkg *source.Package, in *ast.StructType) source.TypeStruct {
	return source.TypeStruct{
		Location:   locationRangeIn(pkg, in, in.Pos(), in.End()),
		Typed:      typedIn(pkg, in),
		Keyword:    locationIn(pkg, in, in.Struct),
		Fields:     loadFieldList(pkg, in.Fields),
		Incomplete: in.Incomplete,
	}
}

func loadVariadic(pkg *source.Package, in *ast.Ellipsis) source.TypeVariadic {
	return source.TypeVariadic{
		Typed:    typedIn(pkg, in),
		Location: locationRangeIn(pkg, in, in.Pos(), in.End()),
		ElementType: source.WithLocation[source.Type]{
			Value:          loadType(pkg, in.Elt),
			SourceLocation: locationIn(pkg, in, in.Ellipsis),
		},
	}
}
