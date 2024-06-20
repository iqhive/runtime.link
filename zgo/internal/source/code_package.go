package source

import (
	"fmt"
	"go/token"
	"go/types"
	"io"
)

type Package struct {
	types.Info

	Name  string
	Test  bool
	Files []File

	fset *token.FileSet
}

func (pkg *Package) compile(out io.Writer) error {
	fmt.Fprintf(out, "const std = @import(\"std\");\n")
	fmt.Fprintf(out, "const go = @import(\"go.zig\");\n")
	for _, f := range pkg.Files {
		if err := f.Compile(out); err != nil {
			return err
		}
	}
	return nil
}

func (pkg *Package) location(pos token.Pos) Location {
	return Location{
		fset: pkg.fset,
		open: pos,
		shut: pos,
	}
}

func (pkg *Package) locations(pos, end token.Pos) Location {
	return Location{
		fset: pkg.fset,
		open: pos,
		shut: end,
	}
}

func (location Location) Errorf(format string, args ...interface{}) error {
	return fmt.Errorf(location.String()+": "+format, args...)
}
