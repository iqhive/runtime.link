package source

import (
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"slices"
)

type Package struct {
	types.Info

	Name  string
	Test  bool
	Files []File
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
