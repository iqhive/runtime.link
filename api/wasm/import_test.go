//go:build wasm

package wasm_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unsafe"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/wasm"
	"github.com/iqhive/runtime.link/api/wasm/internal/example"
	"github.com/iqhive/runtime.link/api/wasm/internal/ffi"
)

var FFI = wasm.Import[ffi.API]()
var Example = wasm.Import[example.API]()

func TestExample(m *testing.T) {
	Example.HelloWorld()
	fmt.Println(Example.HostArch())
}

func TestIntegerArguments(t *testing.T) {
	if got := Example.Add(40, 2); got != 42 {
		t.Fatalf("Add = %d", got)
	}
	if got := Example.Add(-40, 2); got != -38 {
		t.Fatalf("Add = %d", got)
	}
}

func TestEveryScalarKindCrosses(t *testing.T) {
	got := Example.Scalars(-1, 65535, true, 1.5, 2+3i, 7)
	if want := "-1 65535 true 1.5 (2+3i) 7"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestStringArguments(t *testing.T) {
	for _, text := range []string{"", "hello", "h\x00llo w\u00f6rld|&\\", strings.Repeat("long", 1000)} {
		if got := Example.Echo(text); got != text {
			t.Fatalf("Echo(%q) = %q", text, got)
		}
	}
}

func TestDefinedTypesCross(t *testing.T) {
	if got := Example.Warm(20); got != 21.5 {
		t.Fatalf("Warm = %v", got)
	}
}

func TestContextStructAndArrayArguments(t *testing.T) {
	got, err := Example.Move(context.Background(), example.Point{X: 1, Y: 2, Name: "a"}, [2]int32{3, 4})
	if err != nil {
		t.Fatal(err)
	}
	if want := (example.Point{X: 4, Y: 6, Name: "a"}); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSliceArguments(t *testing.T) {
	if got := Example.Join(", ", []string{"a", "b", "c"}); got != "a, b, c" {
		t.Fatalf("Join = %q", got)
	}
	if got := Example.Join(", ", []string{}); got != "" {
		t.Fatalf("Join of an empty slice = %q", got)
	}
	if got := Example.Join(", ", nil); got != "<nil>" {
		t.Fatalf("Join of a nil slice = %q", got)
	}
}

func TestVariadicArguments(t *testing.T) {
	if got := Example.Sum(1, 2, 3); got != 6 {
		t.Fatalf("Sum = %d", got)
	}
	if got := Example.Sum(); got != 0 {
		t.Fatalf("Sum = %d", got)
	}
}

func TestHostErrorReachesTheGuest(t *testing.T) {
	if err := Example.Check("ok"); err != nil {
		t.Fatal(err)
	}
	if err := Example.Check("not ok"); err == nil || err.Error() != "not ok" {
		t.Fatalf("err = %v", err)
	}
}

func TestComplexResult(t *testing.T) {
	if got := Example.Conjugate(1 + 2i); got != 1-2i {
		t.Fatalf("Conjugate = %v", got)
	}
}

func TestSeveralResults(t *testing.T) {
	if head, tail := Example.Split("left right"); head != "left" || tail != "right" {
		t.Fatalf("Split = %q, %q", head, tail)
	}
}

func TestReferenceArgumentsAreRefusedByName(t *testing.T) {
	wantRefusal := func(t *testing.T, name string, call func()) {
		t.Helper()
		defer func() {
			if got := fmt.Sprint(recover()); !strings.Contains(got, name) {
				t.Fatalf("panic = %q, want it to name %s", got, name)
			}
		}()
		call()
	}
	wantRefusal(t, "map[string]int", func() { Example.Keys(map[string]int{"a": 1}) })
	wantRefusal(t, "*example.Point", func() { Example.Mirror(&example.Point{}) })
}

//go:wasmimport example Echo
func direct_echo(text ffi.String) ffi.String

//go:wasmimport github.com/iqhive/runtime.link/ffi string_new
func direct_string_new(data uint64, n uint32) ffi.String

func TestDirectImportTakesAStringHandle(t *testing.T) {
	data := []byte("direct")
	handle := direct_string_new(uint64(uintptr(unsafe.Pointer(unsafe.SliceData(data)))), uint32(len(data)))
	defer FFI.String.Free(handle)
	echoed := direct_echo(handle)
	defer FFI.String.Free(echoed)
	if got := FFI.String.Len(echoed); got != uint32(len(data)) {
		t.Fatalf("Len = %d", got)
	}
}

type absentLibrary struct {
	api.Specification `api:"no such library"`

	Call func()
}

type absentFunction struct {
	api.Specification `api:"example"`

	NoSuchFunction func()
}

func wantPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("calling a function that failed to link should panic in the guest")
		}
	}()
	fn()
}

func TestAbsentLibraryFailsWhenCalled(t *testing.T) {
	absent := wasm.Import[absentLibrary]()
	wantPanic(t, absent.Call)
}

func TestAbsentFunctionFailsWhenCalled(t *testing.T) {
	absent := wasm.Import[absentFunction]()
	wantPanic(t, absent.NoSuchFunction)
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
