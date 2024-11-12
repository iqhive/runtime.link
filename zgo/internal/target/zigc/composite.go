package zigc

import (
	"fmt"
	"go/types"

	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func (zig Target) DataComposite(data source.DataComposite) error {
	dtype, ok := data.Type.Get()
	if !ok {
		return data.Errorf("composite literal missing type")
	}
	fmt.Fprintf(zig, "%s", zig.Type(dtype))
	switch typ := dtype.TypeAndValue().Type.Underlying().(type) {
	case *types.Array:
		fmt.Fprintf(zig, "{")
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(zig, ", ")
			}
			if err := zig.Compile(elem); err != nil {
				return err
			}
		}
		fmt.Fprintf(zig, "}")
		return nil
	case *types.Slice:
		fmt.Fprintf(zig, ".literal(goto, %d, .", len(data.Elements))
		fmt.Fprintf(zig, "{")
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(zig, ", ")
			}
			if err := zig.Compile(elem); err != nil {
				return err
			}
		}
		fmt.Fprintf(zig, "})")
		return nil
	case *types.Map:
		fmt.Fprintf(zig, ".literal(goto, %d, .", len(data.Elements))
		fmt.Fprintf(zig, "{")
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(zig, ", ")
			}
			pair := source.Expressions.KeyValue.Get(elem)
			fmt.Fprintf(zig, ".{")
			if err := zig.Compile(pair.Key); err != nil {
				return err
			}
			fmt.Fprintf(zig, ", ")
			if err := zig.Compile(pair.Value); err != nil {
				return err
			}
			fmt.Fprintf(zig, "}")
		}
		fmt.Fprintf(zig, "})")
		return nil
	case *types.Struct:
		fmt.Fprintf(zig, "{")
		for i, elem := range data.Elements {
			if i > 0 {
				fmt.Fprintf(zig, ", ")
			}
			switch xyz.ValueOf(elem) {
			case source.Expressions.KeyValue:
				if err := zig.Compile(elem); err != nil {
					return err
				}
			default:
				field := typ.Field(i)
				fmt.Fprintf(zig, ".%s = ", field.Name())
				if err := zig.Compile(elem); err != nil {
					return err
				}
			}
		}
		fmt.Fprintf(zig, "}")
		return nil
	default:
		return data.Errorf("unexpected composite type: " + typ.String())
	}
}
