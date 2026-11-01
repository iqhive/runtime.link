package api_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
	"github.com/iqhive/runtime.link/xyz"
)

type failure api.Error[struct {
	Internal xyz.Case[failure, error] `http:"500"
		internal server error`
}]

var failures = xyz.AccessorFor(failure.Values)

type rootedAPI struct {
	api.Specification

	Error api.Register[error, failure]

	Top   func(context.Context) error `rest:"GET /top"`
	Outer struct {
		Middle func(context.Context) error `rest:"GET /middle"`
		Inner  struct {
			Deep func(context.Context) error `rest:"GET /deep"`
		}
	}
}

func TestFunctionRootIsTheOutermostStructure(t *testing.T) {
	var a rootedAPI
	s := api.StructureOf(&a)
	for fn := range s.Iter() {
		if fn.Root.Name != s.Name {
			t.Errorf("%s: Root is %q, want %q", fn, fn.Root.Name, s.Name)
		}
		if len(fn.Root.Scenarios) != len(s.Scenarios) {
			t.Errorf("%s: sees %d scenarios, want %d", fn, len(fn.Root.Scenarios), len(s.Scenarios))
		}
		if len(fn.Root.Instances[reflect.TypeFor[error]()]) != 1 {
			t.Errorf("%s: cannot see the error registered on the root", fn)
		}
		if len(fn.Root.Functions) != 1 || fn.Root.Functions[0].Name != "Top" {
			t.Errorf("%s: Root.Functions = %v", fn, fn.Root.Functions)
		}
	}
}

// The text of an internal error is written for the server's logs, a client must
// be sent the scenario's documentation at every depth of the API.
func TestNestedEndpointsRedactInternalErrors(t *testing.T) {
	var a rootedAPI
	fail := func(context.Context) error { return failures.Internal.As(errors.New("db password=hunter2")) }
	a.Top, a.Outer.Middle, a.Outer.Inner.Deep = fail, fail, fail
	h, err := rest.Handler(nil, &a)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/top", "/middle", "/deep"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if body := strings.TrimSpace(rec.Body.String()); rec.Code != 500 || body != "internal server error" {
			t.Errorf("GET %s = %d %q", path, rec.Code, body)
		}
	}
}

type namespaceRegisteredAPI struct {
	api.Specification

	Top    func(context.Context) error `rest:"GET /top"`
	Nested struct {
		Error api.Register[error, failure]

		Deep func(context.Context) error `rest:"GET /deep"`
	}
}

// Moving Root to the outermost structure must not lose what a namespace
// registered for itself, or its endpoints would start sending raw error text.
func TestNamespaceRegistrationStillRedacts(t *testing.T) {
	var a namespaceRegisteredAPI
	a.Nested.Deep = func(context.Context) error { return failures.Internal.As(errors.New("db password=hunter2")) }
	h, err := rest.Handler(nil, &a)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/deep", nil))
	if body := strings.TrimSpace(rec.Body.String()); rec.Code != 500 || body != "internal server error" {
		t.Errorf("GET /deep = %d %q", rec.Code, body)
	}
	s := api.StructureOf(&a)
	if len(s.Scenarios) != 0 || len(s.Instances) != 0 {
		t.Errorf("registrations of a namespace leaked into the root: %d scenarios, %d instances", len(s.Scenarios), len(s.Instances))
	}
}

func TestStructureOfNilPointer(t *testing.T) {
	type T struct {
		api.Specification
		F func()
	}
	s := api.StructureOf((*T)(nil))
	if s.Name != "T" || len(s.Functions) != 1 || s.Functions[0].Name != "F" {
		t.Errorf("nil *T: name %q, functions %v", s.Name, s.Functions)
	}
	var p *T
	if s := api.StructureOf(&p); s.Name != "T" || len(s.Functions) != 1 {
		t.Errorf("pointer to nil *T: name %q, functions %v", s.Name, s.Functions)
	}
	if s := api.StructureOf(nil); s.Name != "" || len(s.Functions) != 0 {
		t.Errorf("nil: name %q, functions %v", s.Name, s.Functions)
	}
}
