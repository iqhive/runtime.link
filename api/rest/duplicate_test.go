package rest_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

func say(text string) func(context.Context) (string, error) {
	return func(context.Context) (string, error) { return text, nil }
}

func echo(prefix string) func(context.Context, string) (string, error) {
	return func(_ context.Context, v string) (string, error) { return prefix + v, nil }
}

func TestHandlerServesRoutesThatOnlyLookAlike(t *testing.T) {
	type API struct {
		api.Specification
		Get       func(context.Context) (string, error)         `rest:"GET /same"`
		Delete    func(context.Context) (string, error)         `rest:"DELETE /same"`
		Item      func(context.Context, string) (string, error) `rest:"GET /same/{a=%v}"`
		ItemPart  func(context.Context, string) (string, error) `rest:"GET /same/{a=%v}/part"`
		Literal   func(context.Context) (string, error)         `rest:"GET /same/literal"`
		Remainder func(context.Context, string) (string, error) `rest:"GET /rest/{path*=%v}"`
		Wildcard  func(context.Context, string) (string, error) `rest:"GET /rest/{name=%v}/x"`
	}
	impl := API{
		Get:       say("get"),
		Delete:    say("delete"),
		Item:      echo("item:"),
		ItemPart:  echo("part:"),
		Literal:   say("literal"),
		Remainder: echo("remainder:"),
		Wildcard:  echo("wildcard:"),
	}
	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatalf("refused routes a router tells apart: %v", err)
	}
	for _, c := range []struct{ method, target, want string }{
		{"GET", "/same", "get"},
		{"DELETE", "/same", "delete"},
		{"GET", "/same/5", "item:5"},
		{"GET", "/same/5/part", "part:5"},
		{"GET", "/same/literal", "literal"},
		{"GET", "/rest/a/b", "remainder:a/b"},
		{"GET", "/rest/n/x", "wildcard:n"},
	} {
		if rec := serve(t, h, c.method, c.target, "", "Accept", "text/plain"); rec.Body.String() != c.want {
			t.Errorf("%s %s: %d %q, want %q", c.method, c.target, rec.Code, rec.Body, c.want)
		}
	}
	handlers, err := rest.Handlers(nil, impl, "{%s}", "{%s...}")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("registering with http.ServeMux panicked: %v", p)
			}
		}()
		for pattern, handler := range handlers {
			mux.Handle(pattern, handler)
		}
	}()
	if rec := serve(t, mux, "GET", "/same/5", "", "Accept", "text/plain"); rec.Body.String() != "item:5" {
		t.Errorf("GET /same/5 on ServeMux: %d %q", rec.Code, rec.Body)
	}
}

func TestClientLinksAnAPIWithDuplicateRoutes(t *testing.T) {
	// Two functions on one route both call it; only serving one is ambiguous.
	type API struct {
		api.Specification
		One func(context.Context) (string, error) `rest:"GET /same"`
		Two func(context.Context) (string, error) `rest:"GET /same"`
	}
	var client API
	if err := rest.API.Link(api.StructureOf(&client), "http://example.invalid", nil); err != nil {
		t.Errorf("Link = %v", err)
	}
}
