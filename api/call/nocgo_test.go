//go:build !cgo

package call

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unsafe"

	"github.com/iqhive/runtime.link/api"
)

// Every call is made with cgo. Without it, a function was made all the same,
// and it panicked when it was first called.
func TestMakeNeedsCgo(t *testing.T) {
	var target int32 // never called: there is no cgo to call it with.
	fn, err := Make[func(int32) error](unsafe.Pointer(&target), "func(int)int")
	if !errors.Is(err, errNoCgo) || fn != nil {
		t.Errorf("with an error result: got a function that is nil = %v and %v, want an error that says cgo is needed", fn == nil, err)
	}
	plain, err := Make[func(int32) int32](unsafe.Pointer(&target), "func(int)int")
	if !errors.Is(err, errNoCgo) || plain != nil {
		t.Errorf("without an error result: got a function that is nil = %v and %v, want an error that says cgo is needed", plain == nil, err)
	}
}

type needsCgo struct {
	api.Specification
	Abs   func(int32) error `call:"abs func(int)int"`
	Upper func(int32) int32 `call:"toupper func(int)int"`
}

func TestLinkNeedsCgo(t *testing.T) {
	var target int32
	lookup := Options{LookupSymbol: func(string) (unsafe.Pointer, error) { return unsafe.Pointer(&target), nil }}
	var lib needsCgo
	if err := API.Link(api.StructureOf(&lib), "", lookup); !errors.Is(err, errNoCgo) {
		t.Errorf("with a lookup: got %v, want an error that says cgo is needed", err)
	}
	// it is the missing cgo that is the reason, not the library.
	if err := API.Link(api.StructureOf(&lib), "libc.so.6", Options{}); !errors.Is(err, errNoCgo) {
		t.Errorf("with a library: got %v, want an error that says cgo is needed", err)
	}
	imported := api.Import[needsCgo](API, "", lookup)
	if err := imported.Abs(1); !errors.Is(err, errNoCgo) {
		t.Errorf("an imported function with an error result: got %v, want an error that says cgo is needed", err)
	}
	var failure any
	func() {
		defer func() { failure = recover() }()
		imported.Upper(1)
	}()
	if failure == nil || !strings.Contains(fmt.Sprint(failure), "needs cgo") {
		t.Errorf("an imported function without an error result: panic %v, want one that says cgo is needed", failure)
	}
}

// Nothing can call a Back without cgo, so Set leaves it unset, as it does any
// function that foreign code has no way to call.
func TestBackSetWithoutCgoLeavesItUnset(t *testing.T) {
	var back Back[func() int32]
	var failure any
	func() {
		defer func() { failure = recover() }()
		back.Set(func() int32 { return 1 })
	}()
	if failure != nil {
		t.Fatalf("Set panicked: %v", failure)
	}
	if back.pointer() != nil {
		t.Errorf("a callback was made without cgo")
	}
	back.Free()
}
