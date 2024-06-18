package source

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"slices"
)

type Package struct {
	types.Info

	Name  string
	Test  bool
	Files []File

	fset *token.FileSet
}

func (pkg *Package) Compile(out io.Writer) error {
	fmt.Fprintf(out, "const std = @import(\"std\");\n")
	fmt.Fprintf(out, "const go = @import(\"golang.zig\");\n")
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

func Load(dir string, test bool) (map[string]*Package, error) {
	files := token.NewFileSet()
	pkgobj, err := build.ImportDir(dir, 0)
	if err != nil {
		return nil, err
	}
	filter := func(fi os.FileInfo) bool {
		if test {
			return slices.Contains(pkgobj.TestGoFiles, fi.Name())
		}
		return slices.Contains(pkgobj.GoFiles, fi.Name())
	}
	parsed, err := parser.ParseDir(files, dir, filter, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var packages = make(map[string]*Package)
	for name, tree := range parsed {
		var checker = types.Config{
			Importer: importer.Default(),
		}
		var srcPackage = &Package{
			fset: files,

			Name: tree.Name,
			Test: test,
			Info: types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
				Defs:  make(map[*ast.Ident]types.Object),
				Uses:  make(map[*ast.Ident]types.Object),
			},
		}
		pkgFiles := make([]*ast.File, 0, len(tree.Files))
		var keys = make([]string, 0, len(tree.Files))
		for key := range tree.Files {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			file := tree.Files[key]
			pkgFiles = append(pkgFiles, file)
		}
		_, err := checker.Check(name, files, pkgFiles, &srcPackage.Info)
		if err != nil {
			return nil, err
		}
		for _, file := range pkgFiles {
			srcPackage.Files = append(srcPackage.Files, srcPackage.loadFile(file))
		}
		packages[name] = srcPackage
	}
	return packages, nil
}

func (location Location) Errorf(format string, args ...interface{}) error {
	return fmt.Errorf(location.String()+": "+format, args...)
}
