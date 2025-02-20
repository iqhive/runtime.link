//go:build wasm

package wasm_test

import (
	"fmt"
	"testing"

	"runtime.link/api/wasm"
	"runtime.link/api/wasm/internal/example"
)

func TestExample(m *testing.T) {
	Example := wasm.Import[example.API]()
	Example.HelloWorld()
	fmt.Println(Example.HostArch())
}
