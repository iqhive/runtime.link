package source

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
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
	packages, err := load(dir, test)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(".", ".zig"), 0755); err != nil {
		return err
	}
	for _, pkg := range packages {
		out, err := os.Create("./.zig/" + pkg.Name + ".zig") // each Go package is compiled into a single Zig file
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

func load(dir string, test bool) (map[string]Package, error) {
	config := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,

		Tests: true,
	}
	packages, err := packages.Load(config, dir)
	if err != nil {
		return nil, err
	}
	var results = make(map[string]Package)
	for _, pkg := range packages {
		loadPackage(config, results, pkg, test)
	}
	return results, nil
}

func loadPackage(config *packages.Config, into map[string]Package, pkg *packages.Package, test bool) error {
	var loaded = Package{
		Info: *pkg.TypesInfo,
		Name: pkg.Name,
		fset: pkg.Fset,
		Test: test,
	}
	if strings.HasSuffix(pkg.ID, ".test") {
		return nil
	}
	if (strings.HasSuffix(pkg.ID, ".test]") && !test) || (!strings.HasSuffix(pkg.ID, ".test]") && test) {
		return nil
	}
	for _, file := range pkg.Syntax {
		loaded.Files = append(loaded.Files, loaded.loadFile(file))
	}
	into[pkg.Name] = loaded
	for _, imp := range pkg.Imports {
		if strings.HasPrefix(imp.Name, "internal/") {
			continue
		}
		switch imp.Name {
		case "reflect", "testing", "runtime", "os", "syscall", "unsafe", "math":
			continue
		}
		if _, ok := into[imp.Name]; !ok {
			into[imp.Name] = loaded

			packages, err := packages.Load(config, imp.Name)
			if err != nil {
				return err
			}
			for _, pkg := range packages {
				if err := loadPackage(config, into, pkg, false); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
