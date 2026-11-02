package rest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/cors"
	"github.com/iqhive/runtime.link/api/rest"
)

// serve runs one request through h, turning a handler panic into a test failure
// so that one remote panic does not hide every later case.
func serve(t *testing.T, h http.Handler, method, target, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	defer func() {
		if p := recover(); p != nil {
			t.Errorf("%s %s panicked: %v", method, target, p)
		}
	}()
	h.ServeHTTP(rec, req)
	return rec
}

type catalog struct {
	api.Specification

	List   func(context.Context) (string, error)         `rest:"GET /items"`
	Get    func(context.Context, string) (string, error) `rest:"GET /items/{id=%v}"`
	Delete func(context.Context, string) error           `rest:"DELETE /items/{id=%v}"`
	Sub    func(context.Context, string) (string, error) `rest:"GET /items/{id=%v}/sub"`
	Me     func(context.Context) (string, error)         `rest:"GET /users/me"`
	Remove func(context.Context, string) (string, error) `rest:"DELETE /users/{id=%v}"`
	Purge  func(context.Context) (string, error)         `rest:"POST /purge"`
	Hello  func(context.Context) (string, error)         `rest:"GET /purge"`
	Files  func(context.Context, string) (string, error) `rest:"GET /files/{path*=%v}"`
	Search func(context.Context) (string, error)         `rest:"QUERY /search"`
}

// catalogRecord counts the calls with a side effect.
type catalogRecord struct {
	deleted []string
	purged  int
}

func newCatalog(record *catalogRecord) catalog {
	return catalog{
		List: func(context.Context) (string, error) { return "list", nil },
		Get:  func(_ context.Context, id string) (string, error) { return "get:" + id, nil },
		Delete: func(_ context.Context, id string) error {
			record.deleted = append(record.deleted, id)
			return nil
		},
		Sub:    func(_ context.Context, id string) (string, error) { return "sub:" + id, nil },
		Me:     func(context.Context) (string, error) { return "me", nil },
		Remove: func(_ context.Context, id string) (string, error) { return "removed:" + id, nil },
		Purge: func(context.Context) (string, error) {
			record.purged++
			return "purged", nil
		},
		Hello:  func(context.Context) (string, error) { return "hello", nil },
		Files:  func(_ context.Context, p string) (string, error) { return "file:" + p, nil },
		Search: func(context.Context) (string, error) { return "searched", nil },
	}
}

func catalogHandler(t *testing.T, record *catalogRecord) http.Handler {
	t.Helper()
	h, err := rest.Handler(nil, newCatalog(record))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestMethodNameInPathIsNotARoute(t *testing.T) {
	var record catalogRecord
	h := catalogHandler(t, &record)
	for _, target := range []string{"/items/5/DELETE", "/items/5/GET", "/purge/POST", "/purge/GET", "/search/QUERY"} {
		if rec := serve(t, h, "GET", target, ""); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: got %d %q, want 404", target, rec.Code, rec.Body)
		}
	}
	if len(record.deleted) != 0 || record.purged != 0 {
		t.Errorf("a path segment selected a method: deleted=%v purged=%d", record.deleted, record.purged)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[catalog](rest.API, srv.URL, srv.Client())
	got, err := client.Get(context.Background(), "5/DELETE")
	if err != nil || got != "get:5/DELETE" {
		t.Errorf("client.Get: %q, %v", got, err)
	}
	if len(record.deleted) != 0 {
		t.Errorf("a read with a chosen id ran Delete: %v", record.deleted)
	}
}

func TestMethodOverrideReachesQueryRoutes(t *testing.T) {
	var record catalogRecord
	h := catalogHandler(t, &record)
	rec := serve(t, h, "POST", "/search", "", "X-HTTP-Method-Override", "QUERY")
	if rec.Code != 200 || rec.Body.String() != "searched" {
		t.Errorf("POST with override: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "QUERY", "/search", ""); rec.Code != 200 {
		t.Errorf("QUERY: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "GET", "/search", "", "X-HTTP-Method-Override", "QUERY"); rec.Code == 200 && rec.Body.String() == "searched" {
		t.Errorf("the override applied to a GET")
	}
	if rec := serve(t, h, "POST", "/search", "", "X-HTTP-Method-Override", "DELETE"); rec.Body.String() == "searched" {
		t.Errorf("an override other than QUERY was honoured")
	}
}

func TestEmptySegmentNeverFillsAWildcard(t *testing.T) {
	var record catalogRecord
	h := catalogHandler(t, &record)
	for _, tc := range []struct{ method, target string }{
		{"DELETE", "/items/"},
		{"GET", "/items//sub"},
		{"GET", "/items//anything/at/all"},
		{"GET", "/items//"},
		{"DELETE", "/users/"},
		{"GET", "//items"},
		{"GET", "/files/"},
		{"GET", "/files//a"},
	} {
		if rec := serve(t, h, tc.method, tc.target, ""); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: got %d %q, want 404", tc.method, tc.target, rec.Code, rec.Body)
		}
	}
	if len(record.deleted) != 0 {
		t.Errorf("an empty segment reached Delete: %q", record.deleted)
	}
}

func TestTrailingSlashStillMeansTheResource(t *testing.T) {
	var record catalogRecord
	h := catalogHandler(t, &record)
	if rec := serve(t, h, "GET", "/items/", ""); rec.Code != 200 || rec.Body.String() != "list" {
		t.Errorf("GET /items/: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "GET", "/items/5/", ""); rec.Code != 200 || rec.Body.String() != "get:5" {
		t.Errorf("GET /items/5/: %d %q", rec.Code, rec.Body)
	}
}

func TestEscapedSlashStaysInOneSegment(t *testing.T) {
	var record catalogRecord
	h := catalogHandler(t, &record)
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[catalog](rest.API, srv.URL, srv.Client())
	for _, id := range []string{"a/b", "a/sub", "/", "a b", "100%", "?x=1", "#frag"} {
		got, err := client.Get(context.Background(), id)
		if err != nil || got != "get:"+id {
			t.Errorf("client.Get(%q): %q, %v", id, got, err)
		}
	}
	if rec := serve(t, h, "GET", "/items/a%2Fb/sub", ""); rec.Body.String() != "sub:a/b" {
		t.Errorf("GET /items/a%%2Fb/sub: %d %q", rec.Code, rec.Body)
	}
}

func TestRemainderParameterCarriesSlashes(t *testing.T) {
	var record catalogRecord
	h := catalogHandler(t, &record)
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[catalog](rest.API, srv.URL, srv.Client())
	for _, value := range []string{"file.txt", "dir/sub/file.txt", "dir/a b/c?d#e.txt"} {
		got, err := client.Files(context.Background(), value)
		if err != nil || got != "file:"+value {
			t.Errorf("client.Files(%q): %q, %v", value, got, err)
		}
	}
	for target, want := range map[string]string{
		"/files/a//b":    "file:a//b",
		"/files/a%2Fb/c": "file:a/b/c",
		// A trailing slash is no more part of a remainder than of any
		// other path, as it never was.
		"/files/dir/":     "file:dir",
		"/files/dir/sub/": "file:dir/sub",
	} {
		if rec := serve(t, h, "GET", target, ""); rec.Body.String() != want {
			t.Errorf("GET %s: %d %q, want %q", target, rec.Code, rec.Body, want)
		}
	}
}

func TestHandlersFormatRemainders(t *testing.T) {
	var record catalogRecord
	for remainder, want := range map[string]string{
		"*%s":     "GET /files/*path",
		"*":       "GET /files/*",
		"{%s...}": "GET /files/{path...}",
	} {
		handlers, err := rest.Handlers(nil, newCatalog(&record), ":%s", remainder)
		if err != nil {
			t.Fatal(err)
		}
		var patterns []string
		for pattern := range handlers {
			patterns = append(patterns, pattern)
		}
		if !slices.Contains(patterns, want) {
			t.Errorf("remainder format %q: no pattern %q in %q", remainder, want, patterns)
		}
	}
}

// TestHandlersOnServeMux serves the patterns from http.ServeMux, which panics
// on a pattern it has seen before or a wildcard name it cannot parse.
func TestHandlersOnServeMux(t *testing.T) {
	var record catalogRecord
	handlers, err := rest.Handlers(nil, newCatalog(&record), "{%s}", "{%s...}")
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]int)
	mux := http.NewServeMux()
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("registering with http.ServeMux panicked: %v", p)
			}
		}()
		for pattern, h := range handlers {
			seen[pattern]++
			mux.Handle(pattern, h)
		}
	}()
	for pattern, n := range seen {
		if n > 1 {
			t.Errorf("pattern %q yielded %d times", pattern, n)
		}
	}
	if rec := serve(t, mux, "GET", "/files/a/b.txt", ""); rec.Body.String() != "file:a/b.txt" {
		t.Errorf("GET /files/a/b.txt on ServeMux: %d %q", rec.Code, rec.Body)
	}
}

func TestLiteralSegmentFallsBackToWildcard(t *testing.T) {
	var record catalogRecord
	h := catalogHandler(t, &record)
	if rec := serve(t, h, "DELETE", "/users/me", ""); rec.Code != 200 || rec.Body.String() != "removed:me" {
		t.Errorf("DELETE /users/me: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "GET", "/users/me", ""); rec.Body.String() != "me" {
		t.Errorf("GET /users/me: %d %q", rec.Code, rec.Body)
	}
	type API struct {
		api.Specification
		Any func(context.Context, string) (string, error) `rest:"POST /{name=%v}"`
	}
	root, err := rest.Handler(nil, API{Any: func(_ context.Context, name string) (string, error) { return name, nil }})
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, root, "POST", "/documentation", ""); rec.Code != 200 || rec.Body.String() != "documentation" {
		t.Errorf("POST /documentation: %d %q", rec.Code, rec.Body)
	}
}

type corsRecorder struct {
	seen *[]string
}

func (corsRecorder) Authenticate(ctx context.Context, r *http.Request, fn api.Function) (context.Context, error) {
	return ctx, nil
}
func (corsRecorder) Authorize(context.Context, *http.Request, api.Function, []reflect.Value) error {
	return nil
}
func (corsRecorder) Redact(ctx context.Context, err error) error { return err }
func (a corsRecorder) CrossOriginResourceSharing(r *http.Request, fn api.Function) cors.AccessControl {
	if a.seen != nil {
		*a.seen = append(*a.seen, r.Method+":"+fn.Name)
	}
	return cors.AccessControl{AllowOrigin: "https://app.example", AllowCredentials: true}
}

func TestPreflightNamesTheFunction(t *testing.T) {
	type API struct {
		api.Specification
		Read  func(context.Context) (string, error) `rest:"GET /thing"`
		Write func(context.Context) error           `rest:"PUT /thing"`
		Solo  func(context.Context) error           `rest:"POST /solo"`
	}
	var seen []string
	h, err := rest.Handler(corsRecorder{seen: &seen}, API{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		target, requested, want string
	}{
		{"/thing", "", "OPTIONS:Read"},
		{"/thing", "GET", "OPTIONS:Read"},
		{"/thing", "PUT", "OPTIONS:Write"},
		{"/solo", "", "OPTIONS:Solo"},
	} {
		seen = nil
		headers := []string{}
		if tc.requested != "" {
			headers = append(headers, "Access-Control-Request-Method", tc.requested)
		}
		serve(t, h, "OPTIONS", tc.target, "", headers...)
		if len(seen) != 1 || seen[0] != tc.want {
			t.Errorf("preflight of %s for %q: CORS saw %q, want %q", tc.target, tc.requested, seen, tc.want)
		}
	}
}

func TestRootRedirectStaysOnThisServer(t *testing.T) {
	var record catalogRecord
	handlers, err := rest.Handlers(nil, newCatalog(&record), "{%s}", "{%s}")
	if err != nil {
		t.Fatal(err)
	}
	var root http.Handler
	for pattern, h := range handlers {
		if pattern == "GET /" {
			root = h
		}
	}
	for uri, want := range map[string]string{
		"/":                    "/documentation",
		"/echannel":            "/echannel/documentation",
		"/echannel/":           "/echannel/documentation",
		"/echannel/?a=b":       "/echannel/documentation",
		"//evil.example/":      "/evil.example/documentation",
		"///evil.example/":     "/evil.example/documentation",
		`/\evil.example/`:      "/%5Cevil.example/documentation",
		"/%5Cevil.example/":    "/%5Cevil.example/documentation",
		"/%2F/evil.example/":   "/%2F/evil.example/documentation",
		"http://evil.example/": "/documentation",
	} {
		req := httptest.NewRequest("GET", "/", nil)
		req.RequestURI = uri
		rec := httptest.NewRecorder()
		root.ServeHTTP(rec, req)
		got := rec.Header().Get("Location")
		if rec.Code != http.StatusFound || got != want {
			t.Errorf("RequestURI %q: %d Location %q, want %q", uri, rec.Code, got, want)
		}
	}
	// Through the router a path with an empty segment is no route at all.
	h, err := rest.Handler(nil, newCatalog(&record))
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "GET", "//evil.example/", ""); strings.HasPrefix(rec.Header().Get("Location"), "//") {
		t.Errorf("open redirect: %d Location %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestTestRunRedirectStaysOnThisServer(t *testing.T) {
	var runs int
	type Docs struct {
		api.Specification
		api.Documentation
	}
	impl := &Docs{
		Documentation: func(ctx context.Context) (api.Examples, error) {
			return &testDocs{TestingFramework: api.TestingFramework{History: &fakeHistory{}}, runs: &runs}, nil
		},
	}
	handlers, err := rest.Handlers(nil, impl, "{%s}", "{%s}")
	if err != nil {
		t.Fatal(err)
	}
	var run http.Handler
	for pattern, h := range handlers {
		if pattern == "POST /testruns/{name}" {
			run = h
		}
	}
	if run == nil {
		t.Fatal("no test run route")
	}
	req := httptest.NewRequest("POST", "/testruns/ExampleThing", nil)
	req.RequestURI = "//evil.example/testruns/ExampleThing"
	req.SetPathValue("name", "ExampleThing")
	rec := httptest.NewRecorder()
	run.ServeHTTP(rec, req)
	if got := rec.Header().Get("Location"); got != "/evil.example/testruns/ExampleThing" {
		t.Errorf("Location %q", got)
	}
}

func TestUploadFileName(t *testing.T) {
	type API struct {
		api.Specification
		Up func(context.Context, string, fs.File) (string, error) `rest:"POST /upload/{name=%v}"`
	}
	h, err := rest.Handler(nil, API{
		Up: func(_ context.Context, _ string, f fs.File) (string, error) {
			defer f.Close()
			info, err := f.Stat()
			if err != nil {
				return "", err
			}
			return info.Name(), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "POST", "/upload/report.pdf", "data"); rec.Body.String() != "report.pdf" {
		t.Errorf("fallback name %q", rec.Body)
	}
	if rec := serve(t, h, "POST", "/upload/..", "data"); rec.Body.String() == ".." {
		t.Errorf("fallback name %q", rec.Body)
	}
	for disposition, want := range map[string]string{
		`attachment; filename="scan.jpg"`:         "scan.jpg",
		`attachment; filename="../../etc/passwd"`: "passwd",
		`attachment; filename="..\..\boot.ini"`:   "boot.ini",
		`attachment; filename=".."`:               "report.pdf",
		`attachment; filename="/"`:                "report.pdf",
	} {
		rec := serve(t, h, "POST", "/upload/report.pdf", "data", "Content-Disposition", disposition)
		if rec.Body.String() != want {
			t.Errorf("Content-Disposition %s: name %q, want %q", disposition, rec.Body, want)
		}
	}
}

func TestPlainFormatPathParameter(t *testing.T) {
	type API struct {
		api.Specification
		One func(context.Context, string) (string, error)      `rest:"GET /path/with/%v"`
		Two func(context.Context, string, int) (string, error) `rest:"GET /two/%v/and/%v"`
	}
	h, err := rest.Handler(nil, API{
		One: func(_ context.Context, s string) (string, error) { return s, nil },
		Two: func(_ context.Context, s string, n int) (string, error) { return fmt.Sprint(s, ":", n), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	if got, err := client.One(context.Background(), "x"); err != nil || got != "x" {
		t.Errorf("client.One: %q, %v", got, err)
	}
	if got, err := client.Two(context.Background(), "x", 2); err != nil || got != "x:2" {
		t.Errorf("client.Two: %q, %v", got, err)
	}
	rec := serve(t, h, "GET", "/documentation", "", "Accept", "application/json")
	var doc struct {
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	for path := range doc.Paths {
		if strings.Contains(path, "%") {
			t.Errorf("documented path %q still holds a format verb", path)
		}
	}
}

// TestDuplicateRoutesLastDeclaredWins pins that two functions on one route are
// accepted, as they always were, with the one declared last serving it. Under
// different wildcard names, where the winner used to depend on map order, the
// last declared wins too, and Handlers yields the route once.
func TestDuplicateRoutesLastDeclaredWins(t *testing.T) {
	type Same struct {
		api.Specification
		One func(context.Context) (string, error) `rest:"GET /same"`
		Two func(context.Context) (string, error) `rest:"GET /same"`
	}
	type Renamed struct {
		api.Specification
		One   func(context.Context, string) (string, error) `rest:"GET /same/{a=%v}"`
		Two   func(context.Context, string) (string, error) `rest:"GET /same/{b=%v}"`
		Other func(context.Context, string) (string, error) `rest:"DELETE /same/{c=%v}"`
	}
	same := Same{
		One: func(context.Context) (string, error) { return "one", nil },
		Two: func(context.Context) (string, error) { return "two", nil },
	}
	renamed := Renamed{
		One:   func(_ context.Context, v string) (string, error) { return "one:" + v, nil },
		Two:   func(_ context.Context, v string) (string, error) { return "two:" + v, nil },
		Other: func(_ context.Context, v string) (string, error) { return "other:" + v, nil },
	}
	for range 32 {
		h, err := rest.Handler(nil, same)
		if err != nil {
			t.Fatalf("two functions on one route refused: %v", err)
		}
		if rec := serve(t, h, "GET", "/same", "", "Accept", "text/plain"); rec.Body.String() != "two" {
			t.Fatalf("GET /same: %d %q", rec.Code, rec.Body)
		}
		h, err = rest.Handler(nil, renamed)
		if err != nil {
			t.Fatalf("two wildcard names on one route refused: %v", err)
		}
		if rec := serve(t, h, "GET", "/same/5", "", "Accept", "text/plain"); rec.Body.String() != "two:5" {
			t.Fatalf("GET /same/5: %d %q", rec.Code, rec.Body)
		}
		if rec := serve(t, h, "DELETE", "/same/5", "", "Accept", "text/plain"); rec.Body.String() != "other:5" {
			t.Fatalf("DELETE /same/5: %d %q", rec.Code, rec.Body)
		}
		handlers, err := rest.Handlers(nil, renamed, "{%s}", "{%s...}")
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
		if rec := serve(t, mux, "GET", "/same/5", "", "Accept", "text/plain"); rec.Body.String() != "two:5" {
			t.Fatalf("GET /same/5 on ServeMux: %d %q", rec.Code, rec.Body)
		}
	}
}

// pathRecorder records what an Auth is shown of each request.
type pathRecorder struct{ seen *[]string }

func (a pathRecorder) record(r *http.Request) {
	*a.seen = append(*a.seen, fmt.Sprintf("path=%s raw=%s url=%s", r.URL.Path, r.URL.RawPath, r.URL))
}

func (a pathRecorder) Authenticate(ctx context.Context, r *http.Request, fn api.Function) (context.Context, error) {
	a.record(r)
	return ctx, nil
}

func (a pathRecorder) Authorize(ctx context.Context, r *http.Request, fn api.Function, args []reflect.Value) error {
	a.record(r)
	return nil
}

func (a pathRecorder) Redact(ctx context.Context, err error) error { return err }

// TestRoutedRequestHasTheRestOfThePath pins what a function's Auth sees of the
// URL behind rest.Handler: the part of the path left once it was routed, which
// for a route that matched is "/", while the query stays. Before a route
// matches, the request is untouched, so the not found handler sees all of it.
func TestRoutedRequestHasTheRestOfThePath(t *testing.T) {
	var seen []string
	h, err := rest.Handler(pathRecorder{&seen}, newCatalog(&catalogRecord{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/items", "/items/", "/items/5/sub", "/files/a/b", "/items?q=1"} {
		seen = nil
		if rec := serve(t, h, "GET", target, "", "Accept", "text/plain"); rec.Code != http.StatusOK {
			t.Errorf("GET %s: %d %q", target, rec.Code, rec.Body)
		}
		want := "path=/ raw=/ url=/"
		if strings.Contains(target, "?") {
			want += "?q=1"
		}
		for _, got := range seen {
			if got != want {
				t.Errorf("GET %s: Auth saw %q, want %q", target, got, want)
			}
		}
		if len(seen) != 2 {
			t.Errorf("GET %s: Auth saw %d requests", target, len(seen))
		}
	}
	type notFound interface {
		SetNotFoundHandler(http.Handler)
	}
	h.(notFound).SetNotFoundHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, r.URL.Path)
	}))
	if rec := serve(t, h, "GET", "/items/5/nope", ""); rec.Body.String() != "/items/5/nope" {
		t.Errorf("not found handler saw %q", rec.Body)
	}
}

// TestRemainderPathValueKeepsItsAsterisk pins the names a remainder's value is
// set under: the name with its asterisk, as code reading r.PathValue had it,
// and without.
func TestRemainderPathValueKeepsItsAsterisk(t *testing.T) {
	var values []string
	auth := pathValues{func(r *http.Request) {
		values = append(values, r.PathValue("path*")+"|"+r.PathValue("path"))
	}}
	h, err := rest.Handler(auth, newCatalog(&catalogRecord{}))
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "GET", "/files/a/b", "", "Accept", "text/plain"); rec.Body.String() != "file:a/b" {
		t.Fatalf("GET /files/a/b: %d %q", rec.Code, rec.Body)
	}
	if len(values) == 0 || values[0] != "a/b|a/b" {
		t.Errorf("path values %q", values)
	}
}

type pathValues struct{ see func(*http.Request) }

func (a pathValues) Authenticate(ctx context.Context, r *http.Request, fn api.Function) (context.Context, error) {
	a.see(r)
	return ctx, nil
}

func (a pathValues) Authorize(context.Context, *http.Request, api.Function, []reflect.Value) error {
	return nil
}

func (a pathValues) Redact(ctx context.Context, err error) error { return err }

// TestAutomaticRoutesLeaveRealOnesAlone spells one route shape with two wildcard
// names. The form and preflight routes made for one spelling must not replace
// the other's: map order decides who is registered last, so it takes many tries.
func TestAutomaticRoutesLeaveRealOnesAlone(t *testing.T) {
	type thing struct {
		Name string `json:"name"`
	}
	type API struct {
		api.Specification
		Get func(context.Context, string) (string, error) `rest:"GET /k/{id=%v}"`
		Put func(context.Context, thing) error            `rest:"PUT /k/{name=$Name}"`
	}
	impl := API{
		Get: func(_ context.Context, id string) (string, error) { return "got:" + id, nil },
		Put: func(context.Context, thing) error { return nil },
	}
	for range 64 {
		h, err := rest.Handler(nil, impl)
		if err != nil {
			t.Fatal(err)
		}
		if rec := serve(t, h, "GET", "/k/5", ""); rec.Body.String() != "got:5" {
			t.Fatalf("GET /k/5: %d %q", rec.Code, rec.Body)
		}
		handlers, err := rest.Handlers(nil, impl, "{%s}", "{%s...}")
		if err != nil {
			t.Fatal(err)
		}
		mux := http.NewServeMux()
		for pattern, handler := range handlers {
			mux.Handle(pattern, handler)
		}
	}
}

func TestRootResourceYieldsOneRootRoute(t *testing.T) {
	type API struct {
		api.Specification
		Do func(context.Context) error `rest:"POST /"`
	}
	handlers, err := rest.Handlers(nil, API{}, "{%s}", "{%s...}")
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]int)
	for pattern := range handlers {
		seen[pattern]++
	}
	if seen["GET /"] != 1 {
		t.Errorf("GET / yielded %d times", seen["GET /"])
	}
}
