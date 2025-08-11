package zigc

import (
	_ "embed"
	"os"
	"path/filepath"

	"runtime.link/zgo/internal/escape"
	"runtime.link/zgo/internal/parser"
)

var (
	//go:embed build_runtime.zig
	runtime string
	//go:embed build.zig
	buildZig string
)

func Build(dir string, test bool) error {
	packages, err := parser.Load(dir, test)
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
		var zig Target
		zig.CurrentPackage = pkg.Name
		zig.Writer = out
		if err := zig.Package(escape.Analysis(pkg)); err != nil {
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	os.WriteFile("./.zig/build.zig", []byte(buildZig), 0644)
	return os.WriteFile("./.zig/go.zig", []byte(runtime), 0644)
}
