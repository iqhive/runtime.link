package gnu_test

import (
	"context"
	"testing"

	"runtime.link/cmd/posix"
	"runtime.link/qnq"
)

func TestCore(t *testing.T) {
	ctx := context.Background()

	var cmd struct {
		cat posix.CatCommand
	}
	if err := qnq.Link(&cmd); err != nil {
		t.Fatal(err)
	}

	var opts = posix.CatOptions{
		//LineNumbers: true,
	}
	if err := cmd.cat.Files(ctx, posix.Paths{"gnu_test.go"}, &opts); err != nil {
		t.Fatal(err)
	}
}
