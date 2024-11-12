package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	_ "embed"

	"runtime.link/api"
	"runtime.link/api/cmdl"

	"runtime.link/zgo/internal/target/zigc"
	"runtime.link/zgo/internal/zig"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go [build/run]")
		return
	}
	switch os.Args[1] {
	case "build":
		if err := build("."); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "test":
		if err := test("."); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "run":
		if err := run("."); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	default:
		fmt.Println("Usage: go [build/run]")
		os.Exit(1)
	}
}

func build(pkg string) error {
	return zigc.Build(pkg, false)
}

func test(pkg string) error {
	if err := zigc.Build(pkg, true); err != nil {
		return err
	}
	Zig := api.Import[zig.Command](cmdl.API, "zig", nil)
	Zig.Test(context.TODO(), ".zig/main.zig")
	return nil
}

func run(pkg string) error {
	if err := build(pkg); err != nil {
		return err
	}
	os.Chdir("./.zig")
	Zig := api.Import[zig.Command](cmdl.API, "zig", nil)
	Zig.Build(context.TODO())
	binary := exec.Command("./zig-out/bin/main")
	binary.Stderr = os.Stderr
	binary.Stdout = os.Stdout
	return binary.Run()
}
