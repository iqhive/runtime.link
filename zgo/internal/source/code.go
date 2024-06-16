package source

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
)

type Package struct {
	types.Info

	Name  string
	Files []File
}

func Load(dir string) (map[string]*Package, error) {
	files := token.NewFileSet()
	parsed, err := parser.ParseDir(files, dir, nil, parser.ParseComments|parser.SkipObjectResolution)
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
			Info: types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
				Defs:  make(map[*ast.Ident]types.Object),
				Uses:  make(map[*ast.Ident]types.Object),
			},
		}
		pkgFiles := make([]*ast.File, 0, len(tree.Files))
		for _, file := range tree.Files {
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
