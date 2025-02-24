//go:build wasm

package wasm_test

import (
	"fmt"
	"testing"

	"runtime.link/api/wasm"
	"runtime.link/api/wasm/internal/example"
	"runtime.link/ffi"
)

var FFI = wasm.Import[ffi.API]()
var Example = wasm.Import[example.API]()

func TestExample(m *testing.T) {
	Example.HelloWorld()
	fmt.Println(Example.HostArch())

	// Test function parameter support
	result := Example.AddWithCallback(5, func(x int) int {
		return x + 1
	})
	if result != 11 { // (5 * 2) + 1
		m.Errorf("AddWithCallback: expected 11, got %d", result)
	}

	// Test function return value support
	formatter := Example.GetFormatter()
	formatted := formatter("test")
	if formatted != "formatted: test" {
		m.Errorf("GetFormatter: expected 'formatted: test', got %s", formatted)
	}
}

//go:wasmimport example HostArch
func host_arch() ffi.String

func BenchmarkDynamicHostArch(m *testing.B) {
	for m.Loop() {
		Example.HostArch()
	}
}

func BenchmarkWasmImportHostArch(m *testing.B) {
	for m.Loop() {
		host_arch()
	}
}
