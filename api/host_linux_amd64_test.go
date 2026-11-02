package api_test

import (
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/call"
)

// library is the example in the documentation of [call.To].
type library struct {
	api.Specification

	linux   call.To `lib:"libc.so.6 libm.so.6"`
	darwin  call.To `lib:"libSystem.dylib"`
	windows call.To `lib:"msvcrt.dll"`

	Abs func(int32) int32 `call:"abs func(int)int"`
}

// placeholders documents every system without naming a library, which leaves
// the library empty, so call links the symbols of the running process.
type placeholders struct {
	api.Specification `lib:"libnot-installed-anywhere.so"`

	linux   call.To
	darwin  call.To
	windows call.To

	Abs func(int32) int32 `call:"abs func(int)int"`
}

func TestEmptyHostTagsLinkTheProcess(t *testing.T) {
	var lib placeholders
	if err := call.API.Link(api.StructureOf(&lib), "", call.Options{}); err != nil {
		t.Fatal(err)
	}
	if got := lib.Abs(-3); got != 3 {
		t.Errorf("got %d", got)
	}
}

func TestDocumentedLibraryExampleLinks(t *testing.T) {
	var lib library
	structure := api.StructureOf(&lib)
	if err := call.API.Link(structure, "", call.Options{}); err != nil {
		t.Fatal(err)
	}
	if got := lib.Abs(-3); got != 3 {
		t.Errorf("got %d", got)
	}
}
