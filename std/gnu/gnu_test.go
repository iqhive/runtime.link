package gnu_test

import (
	"context"
	"testing"

	"runtime.link/cmd"
	"runtime.link/std/gnu"
	"runtime.link/std/posix"
)

func TestCore(t *testing.T) {
	ctx := context.Background()

	var Concatenate = cmd.Import[gnu.Cat]()

	var opts = gnu.CatOptions{
		LineNumbers: true,
	}
	if err := Concatenate.Files(ctx, posix.Paths{"gnu_test.go"}, &opts); err != nil {
		t.Fatal(err)
	}
}
