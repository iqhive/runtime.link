package zigc

import (
	"fmt"
	"go/types"
	"path"
	"strconv"
	"strings"

	"runtime.link/zgo/internal/source"
)

func (zig Target) Specification(spec source.Specification) error {
	node, _ := spec.Get()
	return zig.Compile(node)
}

func (zig Target) SpecificationImport(spec source.SpecificationImport) error {
	return nil
	path, _ := strconv.Unquote(path.Base(spec.Path.Value))
	rename, ok := spec.Name.Get()
	if ok {
		fmt.Fprintf(zig, "const %s = ", rename.String)
	} else {

		fmt.Fprintf(zig, "const %s = ", path)
	}
	fmt.Fprintf(zig, "@import(%q);\n", path+".zig")
	return nil
}

func (zig Target) SpecificationType(spec source.SpecificationType) error {
	fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
	fmt.Fprintf(zig, "const %s = %s;", spec.Name.String, zig.Type(spec.Type))
	if !spec.PackageLevelScope {
		fmt.Fprintf(zig, "go.use(%s);", spec.Name.String)
	}
	fmt.Fprintf(zig, "const @\"%s.(type)\" = go.rtype{", spec.Name.String)
	fmt.Fprintf(zig, ".name=%q,", spec.Name.String)
	kind := kindOf(spec.Type.TypeAndValue().Type)
	fmt.Fprintf(zig, ".kind=go.rkind.%s, ", kind)
	switch rtype := spec.Type.TypeAndValue().Type.(type) {
	case *types.Struct:
		fmt.Fprintf(zig, ".data=go.rdata{.%s=&[_]go.field{", kind)
		for i := range rtype.NumFields() {
			if i > 0 {
				fmt.Fprintf(zig, ", ")
			}
			field := rtype.Field(i)
			fmt.Fprintf(zig, ".{.name=%q,.type=%s,.offset=@offsetOf(%s,\"%[1]s\"),.exported=%v,.embedded=%v}",
				field.Name(), zig.ReflectTypeOf(field.Type()), spec.Name.String, field.Exported(), field.Anonymous())
		}
		fmt.Fprintf(zig, "}}")
	default:
		fmt.Fprintf(zig, ".data=go.rdata{.%s=void{}}", kind)
	}
	fmt.Fprintf(zig, "}")
	if !spec.PackageLevelScope {
		fmt.Fprintf(zig, "; go.use(@\"%s.(type)\")", spec.Name.String)
	}
	fmt.Fprintf(zig, ";")
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

func (zig Target) SpecificationValue(spec source.SpecificationValue) error {
	for i, name := range spec.Names {
		if zig.Tabs > 0 {
			fmt.Fprintf(zig, "\n%s", strings.Repeat("\t", zig.Tabs))
		}
		var value func() error
		var rtype types.Type
		var ztype string
		vtype, ok := spec.Type.Get()
		if !ok && len(spec.Values) == 0 {
			return fmt.Errorf("missing type for value %s", name.String)
		}
		if ok {
			rtype = vtype.TypeAndValue().Type
			ztype = zig.TypeOf(vtype.TypeAndValue().Type)

		} else if len(spec.Values) > 0 {
			rtype = spec.Values[i].TypeAndValue().Type
			ztype = zig.TypeOf(spec.Values[i].TypeAndValue().Type)
		}
		if len(spec.Values) == 0 {
			value = func() error {
				if ztype[0] == '*' {
					fmt.Fprintf(zig, "null")
					return nil
				}
				fmt.Fprintf(zig, "go.zero(%s)", ztype)
				return nil
			}
		} else {
			value = func() error {
				return zig.Expression(spec.Values[i])
			}
			_, isInterface := rtype.Underlying().(*types.Interface)
			if isInterface {
				value = func() error {
					return zig.ExpressionCall(source.ExpressionCall{
						Location:  spec.Location,
						Function:  source.Expressions.Type.As(vtype),
						Arguments: []source.Expression{spec.Values[i]},
					})
				}
			}
		}
		if name.String == "_" {
			fmt.Fprintf(zig, "_ = ")
			if err := value(); err != nil {
				return err
			}
		} else {
			if spec.Const {
				fmt.Fprintf(zig, "const ")
			} else {
				fmt.Fprintf(zig, "var ")
			}
			if err := zig.Identifier(name); err != nil {
				return err
			}
			fmt.Fprintf(zig, ": %s = ", ztype)
			if err := value(); err != nil {
				return err
			}
			if !spec.Const && !spec.PackageLevelScope {
				fmt.Fprintf(zig, ";")
				if err := zig.Identifier(name); err != nil {
					return err
				}
				fmt.Fprintf(zig, "=")
				if err := zig.Identifier(name); err != nil {
					return err
				}
			}
		}
		if zig.Tabs > 0 || spec.PackageLevelScope {
			fmt.Fprintf(zig, ";")
		}
	}
	return nil
}
