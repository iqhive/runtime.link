package gnu_test

import (
	"context"
	"testing"

	"runtime.link/std"
	"runtime.link/std/gnu"
	"runtime.link/std/posix"
)

func TestCore(t *testing.T) {
	ctx := context.Background()

	var cmd struct {
		cat gnu.Cat
	}
	if err := std.Link(&cmd); err != nil {
		t.Fatal(err)
	}

	var opts = gnu.CatOptions{
		LineNumbers: true,
	}
	if err := cmd.cat.Files(ctx, posix.Paths{"gnu_test.go"}, &opts); err != nil {
		t.Fatal(err)
	}
}
