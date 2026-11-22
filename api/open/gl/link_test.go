//go:build cgo && linux

package gl

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/call"
)

type dynamicLinker struct {
	api.Specification

	Symbol func(handle unsafe.Pointer, name string) unsafe.Pointer `call:"dlsym func(&void,&#char)&void"`
}

// standIn returns a C function to link every GL function to, as there is no
// context here to call OpenGL in. It ignores what it is passed and returns 0.
func standIn(t *testing.T) unsafe.Pointer {
	t.Helper()
	libc := api.Import[dynamicLinker](call.API, "libc.so.6 libdl.so.2", call.Options{})
	var yield unsafe.Pointer
	func() {
		defer func() { recover() }() // a function that could not be linked panics, which leaves nothing to stand in.
		yield = libc.Symbol(nil, "sched_yield")
	}()
	if yield == nil {
		t.Skip("requires dlsym and sched_yield from glibc")
	}
	return yield
}

// Every function was left nil by a link that reported no failure, as none of
// them had a call tag. Each has to be one that api/call can make and call.
func TestEveryFunctionIsLinkedAndCanBeCalled(t *testing.T) {
	yield := standIn(t)
	lookups := make(map[string]int)
	GL := api.Import[API](call.API, "", call.Options{LookupSymbol: func(name string) (unsafe.Pointer, error) {
		lookups[name]++
		return yield, nil
	}})
	var total int
	for _, version := range []any{GL.V1, GL.V2, GL.V3, GL.V4} {
		value := reflect.ValueOf(version)
		for i := range value.NumField() {
			fn, name := value.Field(i), value.Type().Field(i).Name
			if fn.Kind() != reflect.Func {
				continue
			}
			total++
			if fn.IsNil() {
				t.Errorf("%s is nil after linking", name)
				continue
			}
			if lookups["gl"+name] != 1 {
				t.Errorf("%s: gl%s was looked up %d times", name, name, lookups["gl"+name])
			}
			args := make([]reflect.Value, fn.Type().NumIn())
			for j := range args {
				args[j] = reflect.Zero(fn.Type().In(j))
			}
			// a function that api/call could not make panics with the reason when it is called.
			func() {
				defer func() {
					if failure := recover(); failure != nil {
						t.Errorf("%s: %v", name, failure)
					}
				}()
				fn.Call(args)
			}()
		}
	}
	if total != len(lookups) {
		t.Errorf("%d symbols were looked up for %d functions", len(lookups), total)
	}
	// GL keeps the function that it is given for debug messages, so it has to be one that C can call.
	func() {
		defer func() {
			if failure := recover(); failure != nil {
				t.Errorf("DebugMessageCallback with a function: %v", failure)
			}
		}()
		GL.DebugMessageCallback(func(DebugSource, DebugType, uint32, DebugSeverity, int32, *byte, unsafe.Pointer) {}, nil)
	}()
}

// A function that the library does not have reports that when it is called.
func TestFunctionThatIsMissingReportsIt(t *testing.T) {
	yield := standIn(t)
	GL := api.Import[API](call.API, "", call.Options{LookupSymbol: func(name string) (unsafe.Pointer, error) {
		if name == "glClear" {
			return yield, nil
		}
		return nil, fmt.Errorf("%s is not in this library", name)
	}})
	GL.Clear(ClearBufferMaskGL_COLOR_BUFFER_BIT)
	var failure any
	func() {
		defer func() { failure = recover() }()
		GL.Flush()
	}()
	if failure == nil || !strings.Contains(fmt.Sprint(failure), "glFlush is not in this library") {
		t.Errorf("Flush: got %v, want the reason that it could not be linked", failure)
	}
}
