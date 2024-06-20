package source

import (
	_ "embed"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
)

var (
	//go:embed build_runtime.zig
	runtime string
	//go:embed build.zig
	buildZig string
	//go:embed build.zig.zon
	buildZon string
)

func Build(dir string, test bool) error {
	packages, err := load(".", test)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, ".zig"), 0755); err != nil {
		return err
	}
	for _, pkg := range packages {
		out, err := os.Create(dir + "/.zig/" + pkg.Name + ".zig") // each Go package is compiled into a single Zig file
		if err != nil {
			return err
		}
		if err := pkg.compile(out); err != nil {
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	os.WriteFile("./.zig/build.zig", []byte(buildZig), 0644)
	os.WriteFile("./.zig/build.zig.zon", []byte(buildZon), 0644)
	return os.WriteFile("./.zig/go.zig", []byte(runtime), 0644)
}

func load(dir string, test bool) (map[string]*Package, error) {
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
