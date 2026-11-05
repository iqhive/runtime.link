//go:build !wasm

package wasm_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api/wasm"
	"github.com/iqhive/runtime.link/api/wasm/internal/example"
)

type runKey struct{}

func packageFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestExample(t *testing.T) {
	ctx := context.WithValue(t.Context(), runKey{}, "run")

	before := packageFiles(t)
	path := filepath.Join(t.TempDir(), "main.wasm")
	cmd := exec.Command("go", "test", "-c", "-o", path)
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if after := packageFiles(t); !slices.Equal(before, after) {
		t.Errorf("building the guest changed the package directory: %v, then %v", before, after)
	}

	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var runner wasm.Runner
	defer func() {
		if err := runner.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	runner.Set(file)
	runner.Add(example.API{
		HelloWorld: func() {
			fmt.Println("Hello, World!")
		},
		HostArch: func() string {
			return runtime.GOARCH
		},
		Add: func(a int, b int) int {
			return a + b
		},
		Echo: func(text string) string { return text },
		Scalars: func(a int8, b uint16, c bool, d float32, e complex64, f uintptr) string {
			return fmt.Sprint(a, b, c, d, e, f)
		},
		Warm: func(c example.Celsius) example.Celsius { return c + 1.5 },
		Move: func(ctx context.Context, from example.Point, by [2]int32) (example.Point, error) {
			if ctx.Value(runKey{}) != "run" {
				return example.Point{}, errors.New("the call lost Run's context")
			}
			from.X += by[0]
			from.Y += by[1]
			return from, nil
		},
		Join: func(sep string, parts []string) string {
			if parts == nil {
				return "<nil>"
			}
			return strings.Join(parts, sep)
		},
		Sum: func(values ...int) int {
			var sum int
			for _, value := range values {
				sum += value
			}
			return sum
		},
		Check: func(text string) error {
			if text == "ok" {
				return nil
			}
			return errors.New(text)
		},
		Split: func(text string) (string, string) {
			head, tail, _ := strings.Cut(text, " ")
			return head, tail
		},
		Conjugate: func(c complex128) complex128 { return complex(real(c), -imag(c)) },
	})
	runner.SetSystemInterface(wasm.SystemInterface{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Args:   []string{"example", "-test.bench", "."},
		Sleep:  time.Sleep,
		NanoTime: func() int64 {
			return time.Now().UnixNano()
		},
		WallTime: func() (int64, int32) {
			now := time.Now()
			return now.Unix(), int32(now.Nanosecond())
		},
	})
	if err := runner.Run(ctx); err != nil {
		t.Fatal(err)
	}

}
