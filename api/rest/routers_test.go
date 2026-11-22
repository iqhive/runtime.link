package rest_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

// plainRouter routes as the routers of Echo v4 and Gin v1.10 do: by its own
// patterns, ":name" for a segment and "*name" for the rest of the path, under
// an optional prefix, and hands the request on as it came, which is all that
// echo.WrapHandler and gin.WrapH do. It sets no path values.
type plainRouter struct {
	prefix string
	routes map[string]http.Handler
}

func (router plainRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path, ok := strings.CutPrefix(r.URL.EscapedPath(), router.prefix)
	if !ok {
		http.NotFound(w, r)
		return
	}
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for pattern, handler := range router.routes {
		method, route, _ := strings.Cut(pattern, " ")
		if method == r.Method && routeTakes(strings.Split(strings.Trim(route, "/"), "/"), segments) {
			handler.ServeHTTP(w, r)
			return
		}
	}
	http.NotFound(w, r)
}

func routeTakes(route, path []string) bool {
	for i, segment := range route {
		switch {
		case strings.HasPrefix(segment, "*"):
			return true
		case i >= len(path):
			return false
		case !strings.HasPrefix(segment, ":") && segment != path[i]:
			return false
		}
	}
	return len(route) == len(path)
}

type routed struct {
	api.Specification

	List   func(context.Context) (string, error)                 `rest:"GET /things"`
	Get    func(context.Context, string) (string, error)         `rest:"GET /things/{id=%v}"`
	Part   func(context.Context, string, string) (string, error) `rest:"GET /things/{id=%v}/parts/{part=%v}"`
	Tagged func(context.Context, []string) (string, error)       `rest:"GET /tagged/{tags=%v}"`
	Keyed  func(context.Context, pathKey) (string, error)        `rest:"GET /keyed/{ID}"`
	Files  func(context.Context, string) (string, error)         `rest:"GET /files/{path*=%v}"`
	Delete func(context.Context, string) (string, error)         `rest:"DELETE /things/{id=%v}"`
}

func newRouted() routed {
	return routed{
		List:   say("all"),
		Get:    echo("id="),
		Part:   func(_ context.Context, id, part string) (string, error) { return id + "/" + part, nil },
		Tagged: func(_ context.Context, tags []string) (string, error) { return strings.Join(tags, "+"), nil },
		Keyed:  func(_ context.Context, key pathKey) (string, error) { return "key=" + key.ID, nil },
		Files:  echo("path="),
		Delete: echo("deleted="),
	}
}

func routerOf(t *testing.T, prefix string) plainRouter {
	t.Helper()
	handlers, err := rest.Handlers(nil, newRouted(), ":%s", "*%s")
	if err != nil {
		t.Fatalf("failed to create handlers: %v", err)
	}
	router := plainRouter{prefix: prefix, routes: make(map[string]http.Handler)}
	for pattern, handler := range handlers {
		router.routes[pattern] = handler
	}
	return router
}

// TestHandlersBehindARouterThatSetsNoPathValues follows the Echo and Gin recipes
// of the package documentation with a router that does what theirs do. Every
// path parameter was read as empty behind one, so a function was called without
// what its path carries: GET /things/5 got no id.
func TestHandlersBehindARouterThatSetsNoPathValues(t *testing.T) {
	router := routerOf(t, "")
	for _, c := range []struct{ method, target, want string }{
		{"GET", "/things", "all"},
		{"GET", "/things/5", "id=5"},
		{"GET", "/things/5/", "id=5"},
		{"GET", "/things/a%2Fb%20c", "id=a/b c"},
		{"GET", "/things/5/parts/6", "5/6"},
		{"GET", "/tagged/a,b%2Cc", "a+b,c"},
		{"GET", "/keyed/k", "key=k"},
		{"GET", "/files/a/b", "path=a/b"},
		{"GET", "/files/a/b/", "path=a/b/"},
		{"DELETE", "/things/5", "deleted=5"},
	} {
		rec := serve(t, router, c.method, c.target, "", "Accept", "text/plain")
		if rec.Code != 200 || rec.Body.String() != c.want {
			t.Errorf("%s %s: %d %q, want 200 %q", c.method, c.target, rec.Code, rec.Body, c.want)
		}
	}

	// Under a prefix the router adds, a pattern is read from the end of the
	// path. One that ends in a remainder has no end to read from, and is left
	// as it was, without a value.
	router = routerOf(t, "/api/v1")
	for target, want := range map[string]string{
		"/api/v1/things/5":         "id=5",
		"/api/v1/things/5/parts/6": "5/6",
		"/api/v1/files/a/b":        "path=",
	} {
		rec := serve(t, router, "GET", target, "", "Accept", "text/plain")
		if rec.Code != 200 || rec.Body.String() != want {
			t.Errorf("GET %s: %d %q, want 200 %q", target, rec.Code, rec.Body, want)
		}
	}
}

// TestHandlersKeepThePathValuesOfTheirRouter checks that a router which sets
// path values is believed: what it set is what the function gets, whatever the
// path of the request looks like.
func TestHandlersKeepThePathValuesOfTheirRouter(t *testing.T) {
	handlers, err := rest.Handlers(nil, newRouted(), "{%s}", "{%s...}")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	for pattern, handler := range handlers {
		method, path, _ := strings.Cut(pattern, " ")
		mux.Handle(method+" /api"+path, handler)
		if pattern == "GET /things/{id}" {
			// Reached under a path that is not the pattern's, with the
			// value the router of the application chose.
			mux.Handle("GET /named/things/5", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.SetPathValue("id", "chosen")
				handler.ServeHTTP(w, r)
			}))
		}
	}
	for target, want := range map[string]string{
		"/api/things/5":   "id=5",
		"/api/files/a/b":  "path=a/b",
		"/api/files/":     "path=",
		"/named/things/5": "id=chosen",
		"/api/tagged/a,b": "a+b",
	} {
		rec := serve(t, mux, "GET", target, "", "Accept", "text/plain")
		if rec.Code != 200 || rec.Body.String() != want {
			t.Errorf("GET %s on http.ServeMux: %d %q, want 200 %q", target, rec.Code, rec.Body, want)
		}
	}
}

type absolute struct {
	api.Specification

	Read   func(context.Context, string) (string, error) `rest:"GET /files/{path*=%v}"`
	Remove func(context.Context, string) error           `rest:"DELETE /files/{path*=%v}"`
}

// TestClientRefusesARemainderThatStartsWithASlash checks that a remainder which
// starts with a slash, as an absolute path does, is an error to the caller and
// is not sent. The two slashes it makes in the path match no route, so reading
// it was a 404, and deleting it returned nil, as a 404 does for a DELETE,
// without the function having been called.
func TestClientRefusesARemainderThatStartsWithASlash(t *testing.T) {
	var removed []string
	client, log := linked(t, absolute{
		Read:   echo("read:"),
		Remove: func(_ context.Context, path string) error { removed = append(removed, path); return nil },
	})
	ctx := context.Background()
	for _, path := range []string{"/etc/hosts", "/", "//x"} {
		if got, err := client.Read(ctx, path); err == nil {
			t.Errorf("Read(%q) = %q, want an error", path, got)
		}
		if err := client.Remove(ctx, path); err == nil {
			t.Errorf("Remove(%q) returned nil", path)
		}
	}
	if sent := log.all(); len(sent) != 0 || len(removed) != 0 {
		t.Fatalf("a remainder that starts with a slash was sent: %v, and removed %q", sent, removed)
	}
	// Slashes anywhere else are part of the value, as they were.
	for _, path := range []string{"etc/hosts", "a//b", "a/"} {
		if got, err := client.Read(ctx, path); err != nil || got != "read:"+path {
			t.Errorf("Read(%q) requested %q: %q, %v", path, log.last(t).path, got, err)
		}
	}
	if err := client.Remove(ctx, "etc/hosts"); err != nil || len(removed) != 1 || removed[0] != "etc/hosts" {
		t.Errorf("Remove(etc/hosts) requested %q: %v, and removed %q", log.last(t).path, err, removed)
	}
}
