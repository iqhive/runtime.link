package gnu_test

import (
	"context"
	"testing"

	"runtime.link/cmd"
	"runtime.link/cmd/std/posix"
)

func TestCore(t *testing.T) {
	ctx := context.Background()

	var exec struct {
		cat posix.CatCommand
	}
	exec.cat = cmd.Import[posix.CatCommand]("cat")

	var opts = posix.CatOptions{
		//LineNumbers: true,
	}
	if err := exec.cat.Files(ctx, posix.Paths{"gnu_test.go"}, &opts); err != nil {
		t.Fatal(err)
	}
}
