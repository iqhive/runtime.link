package gnu_test

import (
	"context"
	"testing"

	"runtime.link/api"
	"runtime.link/api/args"
	"runtime.link/cmd/std/posix"
)

func TestCore(t *testing.T) {
	ctx := context.Background()

	var exec struct {
		cat posix.CatCommand
	}
	exec.cat = api.Import[posix.CatCommand](args.API, "cat", nil)

	var opts = posix.CatOptions{
		//LineNumbers: true,
	}
	if err := exec.cat.Files(ctx, posix.Paths{"gnu_test.go"}, &opts); err != nil {
		t.Fatal(err)
	}
}
