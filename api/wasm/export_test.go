//go:build !wasm

package wasm_test

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"runtime.link/api/wasm"
	"runtime.link/api/wasm/internal/example"
)

func TestExample(t *testing.T) {
	ctx := t.Context()

	cmd := exec.Command("go", "test", "-c", "-o", "main.wasm")
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	file, err := os.ReadFile("main.wasm")
	if err != nil {
		t.Fatal(err)
	}

	var runner wasm.Runner
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
