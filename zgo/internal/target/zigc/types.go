package zigc

import (
	"fmt"
	"go/types"
	"reflect"
	"strings"

	"runtime.link/zgo/internal/source"
)

func (zig Target) Type(e source.Type) string {
	value, _ := e.Get()
	return zig.TypeOf(value.TypeAndValue().Type)
}

func (zig Target) ReflectType(e source.Type) string {
	value, _ := e.Get()
	return zig.ReflectTypeOf(value.TypeAndValue().Type)
}

func (zig Target) TypeUnknown(source.TypeUnknown) error {
	fmt.Fprintf(zig, "unknown")
	return nil
}

func (zig Target) TypeOf(t types.Type) string {
	switch typ := t.(type) {
	case *types.Basic:
		switch typ.Kind() {
		case types.Bool, types.UntypedBool:
			return "bool"
		case types.Int, types.UntypedInt:
			return "go.int"
		case types.Int8:
			return "go.int8"
		case types.Int16:
			return "go.int16"
		case types.Int32, types.UntypedRune:
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
		case types.Float64, types.UntypedFloat:
			return "go.float64"
		case types.String, types.UntypedString:
			return "go.string"
		case types.Complex64:
			return "go.complex64"
		case types.Complex128, types.UntypedComplex:
			return "go.complex128"
		default:
			panic("unsupported basic type " + typ.String())
		}
	case *types.Array:
		return fmt.Sprintf("[%d]%s", typ.Len(), zig.TypeOf(typ.Elem()))
	case *types.Signature:
		var builder strings.Builder
		builder.WriteString("go.func(fn(*const anyopaque,?*go.routine")
		for i := 0; i < typ.Params().Len(); i++ {
			param := typ.Params().At(i)
			builder.WriteString(", ")
			builder.WriteString(zig.TypeOf(param.Type()))
		}
		builder.WriteString(") ")
		if typ.Results().Len() == 0 {
			builder.WriteString("void")
		} else if typ.Results().Len() == 1 {
			builder.WriteString(zig.TypeOf(typ.Results().At(0).Type()))
		} else {
			panic("unsupported function type with multiple results")
		}
		builder.WriteString(")")
		return builder.String()
	case *types.Named:
		if typ.Obj().Pkg() == nil {
			return "@\"go." + typ.Obj().Name() + "\""
		}
		if typ.Obj().Pkg().Name() == zig.CurrentPackage {
			return typ.Obj().Name()
		}
		return "@\"" + typ.Obj().Pkg().Name() + "." + typ.Obj().Name() + "\""
	case *types.Pointer:
		return "go.pointer(" + zig.TypeOf(typ.Elem()) + ")"
	case *types.Slice:
		return "go.slice(" + zig.TypeOf(typ.Elem()) + ")"
	case *types.Chan:
		return "go.chan(" + zig.TypeOf(typ.Elem()) + ")"
	case *types.Map:
		if typ.Key().String() == "string" {
			return "go.smap(" + zig.TypeOf(typ.Elem()) + ")"
		}
		return "go.map(" + zig.TypeOf(typ.Key()) + ", " + zig.TypeOf(typ.Elem()) + ")"
	case *types.Interface:
		if typ.Empty() {
			return "go.any"
		}
		var builder strings.Builder
		builder.WriteString("go.interface(struct{")
		for i := 0; i < typ.NumMethods(); i++ {
			if i > 0 {
				builder.WriteString(", ")
			}
			method := typ.Method(i)
			builder.WriteString(method.Name())
			builder.WriteString(": ")
			builder.WriteString("*const fn(?*go.routine,*const anyopaque")
			mtype := method.Type().(*types.Signature)
			for i := 0; i < mtype.Params().Len(); i++ {
				param := mtype.Params().At(i)
				builder.WriteString(", ")
				builder.WriteString(zig.TypeOf(param.Type()))
			}
			builder.WriteString(") ")
			if mtype.Results().Len() == 0 {
				builder.WriteString("void")
			} else if mtype.Results().Len() == 1 {
				builder.WriteString(zig.TypeOf(mtype.Results().At(0).Type()))
			} else {
				panic("unsupported function type with multiple results")
			}
		}
		builder.WriteString("})")
		return builder.String()
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
			builder.WriteString(zig.TypeOf(field.Type()))
		}
		builder.WriteString("}")
		return builder.String()
	case *types.Tuple:
		return ".{}"
	case nil:
		return "void"
	case *types.Alias:
		return zig.TypeOf(typ.Rhs())
	default:
		panic("unsupported type " + reflect.TypeOf(typ).String())
	}
}

func (zig Target) ReflectTypeOf(t types.Type) string {
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
		if typ.Obj().Pkg() == nil {
			return "&@\"go." + typ.Obj().Name() + ".(type)\""
		}
		if typ.Obj().Pkg().Name() == zig.CurrentPackage {
			return "&@\"" + typ.Obj().Name() + ".(type)\""
		}
		return "&@\"" + typ.Obj().Pkg().Name() + "." + typ.Obj().Name() + ".(type)\""
	case *types.Pointer:
		return "go.rptr(goto, " + zig.ReflectTypeOf(typ.Elem()) + ")"
	default:
		panic("unsupported type " + reflect.TypeOf(typ).String())
	}
}
