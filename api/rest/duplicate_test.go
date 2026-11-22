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

type twins struct {
	api.Specification

	One   func(context.Context) (string, error)         `rest:"GET /same"`
	Two   func(context.Context) (string, error)         `rest:"GET /same"`
	Three func(context.Context, string) (string, error) `rest:"GET /same?said=%v"`
	Other func(context.Context) (string, error)         `rest:"POST /same"`
}

// TestClientLinksAnAPIWithDuplicateRoutes checks that every function of an API
// with several on one route calls it: only serving one is ambiguous. Only the
// one declared last was linked, and calling another was a nil function call.
func TestClientLinksAnAPIWithDuplicateRoutes(t *testing.T) {
	var unlinked twins
	if err := rest.API.Link(api.StructureOf(&unlinked), "http://example.invalid", nil); err != nil {
		t.Errorf("Link = %v", err)
	}
	if unlinked.One == nil || unlinked.Two == nil || unlinked.Three == nil || unlinked.Other == nil {
		t.Fatalf("Link left a function nil: %v", []bool{unlinked.One == nil, unlinked.Two == nil, unlinked.Three == nil, unlinked.Other == nil})
	}
	// The one declared last serves the route, whichever function calls it.
	client, log := linked(t, twins{
		One:   say("one"),
		Two:   say("two"),
		Three: echo("three:"),
		Other: say("other"),
	})
	ctx := context.Background()
	for name, call := range map[string]func(context.Context) (string, error){"One": client.One, "Two": client.Two} {
		if got, err := call(ctx); err != nil || got != "three:" || log.last(t).path != "/same" {
			t.Errorf("%s requested %s %s: %q, %v", name, log.last(t).method, log.last(t).path, got, err)
		}
	}
	if got, err := client.Three(ctx, "x"); err != nil || got != "three:x" || log.last(t).query != "said=x" {
		t.Errorf("Three(x) requested %s?%s: %q, %v", log.last(t).path, log.last(t).query, got, err)
	}
	if got, err := client.Other(ctx); err != nil || got != "other" || log.last(t).method != "POST" {
		t.Errorf("Other requested %s %s: %q, %v", log.last(t).method, log.last(t).path, got, err)
	}
}
