package cgo

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

type platform struct {
	goos, goarch string
}

// platforms that the module is expected to build for with cgo on,
// these have no cross toolchain here, so only the selected files can be checked.
var platforms = []platform{
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"android", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"windows", "amd64"},
}

func selected(t *testing.T, dir string, target platform, cgo bool) *build.Package {
	t.Helper()
	ctx := build.Default
	ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = target.goos, target.goarch, cgo
	pkg, err := ctx.ImportDir(dir, 0)
	if err != nil {
		t.Fatalf("%s/%s cgo=%v: %v", target.goos, target.goarch, cgo, err)
	}
	return pkg
}

func TestDumpAndTablesAreDeclaredOnceOnEveryPlatform(t *testing.T) {
	for _, target := range platforms {
		for _, cgo := range []bool{true, false} {
			pkg := selected(t, ".", target, cgo)
			counts := map[string]int{}
			fset := token.NewFileSet()
			for _, name := range append(pkg.GoFiles, pkg.CgoFiles...) {
				file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
				if err != nil {
					t.Fatal(err)
				}
				for _, decl := range file.Decls {
					switch decl := decl.(type) {
					case *ast.FuncDecl:
						counts[decl.Name.Name]++
					case *ast.GenDecl:
						for _, spec := range decl.Specs {
							if value, ok := spec.(*ast.ValueSpec); ok {
								counts[value.Names[0].Name]++
							}
						}
					}
				}
			}
			for _, name := range []string{"Dump", "Constants", "Types"} {
				if counts[name] != 1 {
					t.Errorf("%s/%s cgo=%v: %s is declared %d times", target.goos, target.goarch, cgo, name, counts[name])
				}
			}
		}
	}
}

func TestEveryPlatformDefinesTheCallVM(t *testing.T) {
	definition := regexp.MustCompile(`(?m)^DCCallVM\* dcNewCallVM\(`)
	for _, target := range platforms {
		pkg := selected(t, "dyncall", target, true)
		var found int
		for _, name := range pkg.CFiles {
			source, err := os.ReadFile(filepath.Join("dyncall", name))
			if err != nil {
				t.Fatal(err)
			}
			if definition.Match(source) {
				found++
			}
		}
		if found != 1 {
			t.Errorf("%s/%s: dcNewCallVM is defined %d times in %v", target.goos, target.goarch, found, pkg.CFiles)
		}
	}
}

func TestOnlyOneExecutableAllocatorIsSelected(t *testing.T) {
	allocator := regexp.MustCompile(`^dyncall_alloc_wx_`)
	for _, target := range platforms {
		var found int
		for _, name := range selected(t, "dyncall", target, true).CFiles {
			if allocator.MatchString(name) {
				found++
			}
		}
		if found != 1 {
			t.Errorf("%s/%s: %d allocators selected", target.goos, target.goarch, found)
		}
	}
}
