//go:build arm64

package arm64_test

import (
	"testing"

	"runtime.link/cpu/arm64"
)

func TestArm64(t *testing.T) {
	fn, err := arm64.Compile[func(int64) int64](
		arm64.Abs(arm64.X0, arm64.X0),
		arm64.Return(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fn(-1) != 1 {
		t.Fatal("unexpected value")
	}
}
