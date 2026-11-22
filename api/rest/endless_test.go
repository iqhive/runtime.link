package rest_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

// endless is a pointer to nothing but itself: there is no value at the end of
// it, and following it never stops.
type endless *endless

type holder struct {
	ID string `json:"id"`
}

type endlessAPI struct {
	api.Specification

	JSON   func(context.Context, endless) (string, error)         `rest:"POST /json"`
	Text   func(context.Context, endless) (string, error)         `rest:"POST(text/plain) /text"`
	List   func(context.Context, []endless) (string, error)       `rest:"POST(text/plain) /list"`
	Path   func(context.Context, endless) (string, error)         `rest:"GET /path/{v=%v}"`
	Query  func(context.Context, endless) (string, error)         `rest:"GET /query?v=%v"`
	Many   func(context.Context, []endless) (string, error)       `rest:"GET /many?v=%v"`
	Beside func(context.Context, endless, holder) (string, error) `rest:"GET /beside/{ID}"`
	Get    func(context.Context) (endless, error)                 `rest:"GET /get"`
	All    func(context.Context) ([]endless, error)               `rest:"GET /all"`
	Both   func(context.Context) (endless, string, error)         `rest:"GET /both a,b"`
}

func newEndlessAPI() endlessAPI {
	said := func(_ context.Context, v endless) (string, error) {
		if v == nil {
			return "nil", nil
		}
		return "set", nil
	}
	many := func(_ context.Context, v []endless) (string, error) { return "many", nil }
	// A value that holds itself, which is the only one of the type there is
	// beside nil.
	var itself endless
	itself = &itself
	return endlessAPI{
		JSON: said, Text: said, Path: said, Query: said, List: many, Many: many,
		Beside: func(_ context.Context, _ endless, h holder) (string, error) { return h.ID, nil },
		Get:    func(context.Context) (endless, error) { return itself, nil },
		All:    func(context.Context) ([]endless, error) { return []endless{itself}, nil },
		Both:   func(context.Context) (endless, string, error) { return itself, "b", nil },
	}
}

// within fails the test when call takes longer than a request ever should: what
// is tested here used to never return.
func within(t *testing.T, name string, call func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		call()
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("%s did not return", name)
	}
}

// TestPointerToItselfIsRefused checks that an argument or result whose type is
// a pointer to nothing but itself is an error where it would have to be read
// or written, and no more than that: reading one from a text/plain body used to
// follow the pointers of the type for ever, as did building the API of a
// function that has one beside a struct of the path, and a JSON body for one
// ended the process, as encoding/json follows it until the stack runs out.
func TestPointerToItselfIsRefused(t *testing.T) {
	var h http.Handler
	within(t, "Handler", func() {
		var err error
		if h, err = rest.Handler(nil, newEndlessAPI()); err != nil {
			t.Error(err)
		}
	})
	if h == nil {
		t.FailNow()
	}
	for _, c := range []struct {
		method, target, body, ctype string
		want                        int
	}{
		{"POST", "/json", "5", "application/json", 400},
		{"POST", "/json", `{"a":1}`, "application/json", 400},
		{"POST", "/json", "null", "application/json", 200},
		{"POST", "/text", "x", "text/plain", 400},
		{"GET", "/beside/k", `[5,{"id":"k"}]`, "application/json", 400},
		{"POST", "/list", "x\ny\n", "text/plain", 400},
		{"GET", "/path/x", "", "", 400},
		{"GET", "/query?v=x", "", "", 400},
		{"GET", "/many?v=x&v=y", "", "", 400},
		// With nothing sent there is nothing to read, and the function gets
		// the nil it always got.
		{"GET", "/query", "", "", 200},
		{"GET", "/many", "", "", 200},
		{"GET", "/get", "", "", 500},
		{"GET", "/both", "", "", 500},
	} {
		within(t, c.method+" "+c.target, func() {
			for _, accept := range []string{"", "text/plain", "application/json", "text/*"} {
				rec := serve(t, h, c.method, c.target, c.body, "Content-Type", c.ctype, "Accept", accept)
				if rec.Code != c.want {
					t.Errorf("%s %s accepting %q: %d %q, want %d", c.method, c.target, accept, rec.Code, rec.Body, c.want)
				}
			}
		})
	}
	within(t, "GET /all", func() {
		if rec := serve(t, h, "GET", "/all", "", "Accept", "text/plain"); rec.Code != 500 {
			t.Errorf("GET /all as text/plain: %d %q, want 500", rec.Code, rec.Body)
		}
	})
	within(t, "GET /beside/k", func() {
		rec := serve(t, h, "GET", "/beside/k", `[null,{"id":"k"}]`, "Content-Type", "application/json", "Accept", "text/plain")
		if rec.Code != 200 || rec.Body.String() != "k" {
			t.Errorf("GET /beside/k: %d %q", rec.Code, rec.Body)
		}
	})

	client, log := linked(t, newEndlessAPI())
	ctx := context.Background()
	var itself endless
	itself = &itself
	for name, call := range map[string]func(context.Context, endless) (string, error){
		"a JSON body": client.JSON, "a text/plain body": client.Text, "a path": client.Path, "a query": client.Query,
	} {
		within(t, "a client sending one in "+name, func() {
			sent := len(log.all())
			if got, err := call(ctx, itself); err == nil || len(log.all()) != sent {
				t.Errorf("a pointer to itself in %s was sent, and answered %q", name, got)
			}
		})
	}
	within(t, "a client sending a list of them", func() {
		if got, err := client.Many(ctx, []endless{itself}); err == nil {
			t.Errorf("a list of pointers to themselves in a query was sent, and answered %q", got)
		}
	})
	// A nil one is nothing to follow, and is sent as nothing, as it was.
	within(t, "a client sending nil", func() {
		if got, err := client.Query(ctx, nil); err != nil || got != "nil" {
			t.Errorf("Query(nil) = %q, %v", got, err)
		}
		if got, err := client.Text(ctx, nil); err != nil || got != "nil" {
			t.Errorf("Text(nil) sent %q: %q, %v", log.last(t).body, got, err)
		}
	})
}
