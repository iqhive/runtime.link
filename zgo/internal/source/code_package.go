package source

import (
	"fmt"
	"go/token"
	"go/types"
	"io"
	"path"
	"strconv"
)

type Package struct {
	types.Info

	Name  string
	Test  bool
	Files []File

	fset *token.FileSet
}

func (pkg *Package) compile(out io.Writer) error {
	fmt.Fprintf(out, "const go = @import(\"go.zig\");\n")
	var imports = make(map[string]bool)
	for _, f := range pkg.Files {
		for _, imp := range f.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return fmt.Errorf("invalid import path: %v", err)
			}
			imports[name] = true
		}
	}
	delete(imports, "testing")
	delete(imports, "math")
	for name := range imports {
		fmt.Println(name)
		fmt.Fprintf(out, `const %s = @import("%s.zig");`+"\n", path.Base(name), name)
	}
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

func zigPackageOf(name string) string {
	if name == "testing" {
		return "go.testing"
	}
	if name == "math" {
		return "go.math"
	}
	return name
}
