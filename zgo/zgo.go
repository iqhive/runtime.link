package zgo

import (
	"context"
	"fmt"
	"go/token"
	"os"
	"os/exec"

	_ "embed"

	"runtime.link/api"
	"runtime.link/api/args"

	"runtime.link/zgo/internal/source"
	"runtime.link/zgo/internal/zig"
)

//go:embed runtime.zig
var runtime string

//go:embed testing.zig
var testing string

//go:embed build.zig
var buildZig string

//go:embed build.zig.zon
var buildZon string

type Compiler struct {
	files *token.FileSet
	out   string // directory
}

func (zgo *Compiler) compile(name string, file []source.File) error {
	out, err := os.Create(zgo.out + "/" + name + ".zig") // each Go package is compiled into a single Zig file
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "const std = @import(\"std\");\n")
	fmt.Fprintf(out, "const runtime = @import(\"runtime.zig\");\n")
	defer out.Close()
	for _, f := range file {
		if err := f.Compile(out); err != nil {
			return err
		}
	}
	return nil
}

func Build() error {
	zgo := Compiler{
		out: ".",
	}
	packages, err := source.Load(".")
	if err != nil {
		return err
	}
	for _, pkg := range packages {
		if err := zgo.compile(pkg.Name, pkg.Files); err != nil {
			return err
		}
	}
	os.WriteFile("testing.zig", []byte(testing), 0644)
	os.WriteFile("build.zig", []byte(buildZig), 0644)
	os.WriteFile("build.zig.zon", []byte(buildZon), 0644)
	return os.WriteFile("runtime.zig", []byte(runtime), 0644)
}

func Run() error {
	if err := Build(); err != nil {
		return err
	}
	Zig := api.Import[zig.Command](args.API, "zig", nil)
	if err := Zig.Init(context.TODO()); err != nil {
		return err
	}
	if err := Zig.Build(context.TODO()); err != nil {
		return err
	}
	binary := exec.Command("./zig-out/bin/main")
	binary.Stderr = os.Stderr
	binary.Stdout = os.Stdout
	return binary.Run()
}
