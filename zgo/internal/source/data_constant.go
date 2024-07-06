package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"strconv"
	"strings"
)

type Constant struct {
	typed

	Location

	WithLocation[string]
	Kind token.Token
}

func (pkg *Package) loadConstant(in *ast.BasicLit) Constant {
	return Constant{
		Location: pkg.locations(in.Pos(), in.End()),
		typed:    pkg.typed(in),
		WithLocation: WithLocation[string]{
			Value:          in.Value,
			SourceLocation: pkg.location(in.ValuePos),
		},
		Kind: in.Kind,
	}
}

func (lit Constant) compile(w io.Writer, tabs int) error {
	if lit.Kind == token.INT && len(lit.Value) > 1 {
		if lit.Value[0] == '0' && ((lit.Value[1] > '0' && lit.Value[1] < '9') || lit.Value[1] == '_') {
			// Zig does not support leading zeroes in integer
			// literals.
			_, err := w.Write([]byte("0o" + strings.TrimPrefix(lit.Value[1:], "_")))
			return err
		}
	}
	if (lit.Kind == token.IMAG || lit.Kind == token.FLOAT) && len(lit.Value) > 1 {
		lit.Value = strings.TrimSuffix(lit.Value, "i")
		if lit.Value == "0" {
			lit.Value = "0.0"
		}
		// Zig does not support leading zeros, decimal points or trailing
		// decimal points in floating point literals.
		if lit.Value[1] != 'x' && lit.Value[1] != 'o' {
			lit.Value = strings.TrimLeft(lit.Value, "0")
		}
		if lit.Value == "." {
			lit.Value = "0.0"
		}
		if lit.Value[0] == '.' {
			lit.Value = "0" + lit.Value
		}
		if lit.Value[len(lit.Value)-1] == '.' {
			lit.Value = lit.Value + "0"
		}
	}
	if lit.Kind == token.IMAG {
		fmt.Fprintf(w, "go.complex128.init(0,%s)", lit.Value)
		return nil
	}
	if lit.Kind == token.CHAR {
		// we just convert runes into integer values.
		value, _, _, err := strconv.UnquoteChar(lit.Value[1:], '\'')
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%d", value)
		return nil
	}
	if lit.Kind == token.STRING {
		// normalize string literals, as zig has a different format for
		// unicode escape sequences.
		val, err := strconv.Unquote(lit.Value)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%q", val)
		return nil
	}
	_, err := w.Write([]byte(strings.ReplaceAll(lit.Value, "_", "")))
	return err
}
