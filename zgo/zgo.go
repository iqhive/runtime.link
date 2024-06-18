package zgo

import (
	"context"
	"go/token"
	"os"
	"os/exec"

	_ "embed"

	"runtime.link/api"
	"runtime.link/api/args"

	"runtime.link/zgo/internal/source"
	"runtime.link/zgo/internal/zig"
)

//go:embed golang.zig
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

func Build() error {
	zgo := Compiler{
		out: ".",
	}
	packages, err := source.Load(".", false)
	if err != nil {
		return err
	}
	for _, pkg := range packages {
		out, err := os.Create(zgo.out + "/" + pkg.Name + ".zig") // each Go package is compiled into a single Zig file
		if err != nil {
			return err
		}
		if err := pkg.Compile(out); err != nil {
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	os.WriteFile("testing.zig", []byte(testing), 0644)
	os.WriteFile("build.zig", []byte(buildZig), 0644)
	os.WriteFile("build.zig.zon", []byte(buildZon), 0644)
	return os.WriteFile("golang.zig", []byte(runtime), 0644)
}

func Test() error {
	zgo := Compiler{
		out: ".",
	}
	packages, err := source.Load(".", true)
	if err != nil {
		return err
	}
	for _, pkg := range packages {
		out, err := os.Create(zgo.out + "/" + pkg.Name + ".zig") // each Go package is compiled into a single Zig file
		if err != nil {
			return err
		}
		if err := pkg.Compile(out); err != nil {
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	os.WriteFile("testing.zig", []byte(testing), 0644)
	os.WriteFile("build.zig", []byte(buildZig), 0644)
	os.WriteFile("build.zig.zon", []byte(buildZon), 0644)
	os.WriteFile("golang.zig", []byte(runtime), 0644)
	Zig := api.Import[zig.Command](args.API, "zig", nil)
	Zig.Test(context.TODO(), "main.zig")
	return nil
}

func Run() error {
	if err := Build(); err != nil {
		return err
	}
	Zig := api.Import[zig.Command](args.API, "zig", nil)
	Zig.Build(context.TODO())
	binary := exec.Command("./zig-out/bin/main")
	binary.Stderr = os.Stderr
	binary.Stdout = os.Stdout
	return binary.Run()
}
