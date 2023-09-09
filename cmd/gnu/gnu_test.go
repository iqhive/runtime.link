package gnu_test

import (
	"context"
	"testing"

	"runtime.link/cmd"
	"runtime.link/cmd/gnu"
)

func TestCore(t *testing.T) {
	ctx := context.Background()

	var nl = cmd.Import[gnu.NumberLines]()

	if err := nl.Print(ctx, &gnu.LineNumbering{
		Body: gnu.LineNumbers.Each,
	}, "./gnu_test.go"); err != nil {
		t.Fatal(err)
	}
}
