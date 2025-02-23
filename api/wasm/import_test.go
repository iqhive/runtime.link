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
