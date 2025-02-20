//go:build !wasm

package wasm_test

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"runtime.link/api/wasm"
	"runtime.link/api/wasm/internal/example"

	"github.com/tetratelabs/wazero"
)

func TestExample(t *testing.T) {
	ctx := t.Context()

	// Create a new WebAssembly Runtime.
	r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig())
	defer r.Close(ctx) // This closes everything this Runtime created.

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

	buf, err := wasm.CombinedOutput(ctx, file, example.API{
		HelloWorld: func() {
			fmt.Println("Hello, World!")
		},
		HostArch: func() string {
			return runtime.GOARCH
		},
	})
	fmt.Print(string(buf))
	if err != nil {
		t.Fatal(err)
	}

}
