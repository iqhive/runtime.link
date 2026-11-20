package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

type deniedError struct{}

func (deniedError) Error() string   { return "denied" }
func (deniedError) StatusHTTP() int { return http.StatusUnauthorized }

// tokenAuth accepts only requests carrying X-Token: ok, and refuses the
// functions named in hidden whoever asks.
type tokenAuth struct {
	hidden []string
}

func (a tokenAuth) Authenticate(ctx context.Context, r *http.Request, fn api.Function) (context.Context, error) {
	for _, name := range a.hidden {
		if fn.Name == name {
			return ctx, deniedError{}
		}
	}
	if r.Header.Get("X-Token") == "ok" {
		return ctx, nil
	}
	return ctx, deniedError{}
}
func (tokenAuth) Authorize(context.Context, *http.Request, api.Function, []reflect.Value) error {
	return nil
}
func (tokenAuth) Redact(ctx context.Context, err error) error { return err }

func TestPagesRequireAuthentication(t *testing.T) {
	var runs int
	type Docs struct {
		api.Specification
		api.Documentation
	}
	history := &fakeHistory{}
	impl := &Docs{
		Documentation: func(ctx context.Context) (api.Examples, error) {
			return &testDocs{TestingFramework: api.TestingFramework{History: history}, runs: &runs}, nil
		},
	}
	h, err := rest.Handler(tokenAuth{}, impl)
	if err != nil {
		t.Fatal(err)
	}
	pages := []struct{ method, target, accept string }{
		{"GET", "/documentation", "text/html"},
		{"GET", "/documentation", "application/json"},
		{"GET", "/documentation", "application/javascript"},
		{"POST", "/testruns/ExampleThing", "text/html"},
		{"GET", "/testruns", "text/html"},
		{"GET", "/testruns", ""},
		{"GET", "/testruns/ExampleThing", "text/html"},
		{"GET", "/examples/ExampleThing", "text/html"},
		{"GET", "/examples/ExampleThing", ""},
	}
	for _, page := range pages {
		rec := serve(t, h, page.method, page.target, "", "Accept", page.accept)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s (%s): %d", page.method, page.target, page.accept, rec.Code)
		}
	}
	if runs != 0 {
		t.Errorf("an anonymous caller ran a test or example %d times", runs)
	}
	for _, page := range pages {
		rec := serve(t, h, page.method, page.target, "", "Accept", page.accept, "X-Token", "ok")
		if rec.Code == http.StatusUnauthorized {
			t.Errorf("signed in %s %s (%s): %d", page.method, page.target, page.accept, rec.Code)
		}
	}
	if runs == 0 {
		t.Errorf("a signed in caller could not run a test")
	}
}

func TestFormsRequireAuthentication(t *testing.T) {
	type thing struct {
		Name string `json:"name"`
	}
	type API struct {
		api.Specification
		Create func(context.Context, thing) error `rest:"POST /create"
			internal only: creates a thing in the admin namespace`
		Mixed func(context.Context, thing) error `rest:"POST /mixed"
			shown to the caller`
		Hidden func(context.Context, thing) error `rest:"PUT /mixed"
			kept from the caller`
	}
	h, err := rest.Handler(tokenAuth{hidden: []string{"Hidden"}}, API{})
	if err != nil {
		t.Fatal(err)
	}
	rec := serve(t, h, "GET", "/create?method=POST", "", "Accept", "application/schema+json")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous schema: %d %q", rec.Code, rec.Body)
	}
	rec = serve(t, h, "GET", "/create", "", "Accept", "text/html")
	if rec.Code != http.StatusUnauthorized || strings.Contains(rec.Body.String(), "internal only") {
		t.Errorf("anonymous form: %d %q", rec.Code, rec.Body)
	}
	rec = serve(t, h, "GET", "/create?method=POST", "", "Accept", "application/schema+json", "X-Token", "ok")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "name") {
		t.Errorf("signed in schema: %d %q", rec.Code, rec.Body)
	}
	rec = serve(t, h, "GET", "/mixed", "", "Accept", "text/html", "X-Token", "ok")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "shown to the caller") {
		t.Errorf("signed in form: %d %q", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "kept from the caller") {
		t.Errorf("the form showed a function Authenticate refused")
	}
	if rec := serve(t, h, "OPTIONS", "/create", ""); rec.Code == http.StatusUnauthorized {
		t.Errorf("a preflight carries no credentials, it cannot need them: %d", rec.Code)
	}
}

func TestNilAuthLeavesPagesOpen(t *testing.T) {
	var runs int
	type Docs struct {
		api.Specification
		api.Documentation
	}
	history := &fakeHistory{}
	impl := &Docs{
		Documentation: func(ctx context.Context) (api.Examples, error) {
			return &testDocs{TestingFramework: api.TestingFramework{History: history}, runs: &runs}, nil
		},
	}
	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "GET", "/examples/ExampleThing", ""); rec.Code != 200 {
		t.Errorf("GET /examples/ExampleThing: %d", rec.Code)
	}
	if rec := serve(t, h, "GET", "/testruns", ""); rec.Code != 200 {
		t.Errorf("GET /testruns: %d", rec.Code)
	}
}

func TestDocumentationOmitsRefusedFunctions(t *testing.T) {
	type Deep struct {
		Buried func(context.Context) error `rest:"GET /admin/deep/buried"`
	}
	type Admin struct {
		Secret func(context.Context) error `rest:"GET /admin/secret"`
		Open   func(context.Context) error `rest:"GET /admin/open"`
		Deep   Deep
	}
	type API struct {
		api.Specification
		Public func(context.Context) error `rest:"GET /public"`
		Hidden func(context.Context) error `rest:"GET /hidden"`
		Admin  Admin
	}
	h, err := rest.Handler(tokenAuth{hidden: []string{"Hidden", "Secret", "Buried"}}, API{})
	if err != nil {
		t.Fatal(err)
	}
	rec := serve(t, h, "GET", "/documentation", "", "Accept", "application/json", "X-Token", "ok")
	var doc struct {
		Paths map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("%v: %q", err, rec.Body)
	}
	for _, path := range []string{"/hidden", "/admin/secret", "/admin/deep/buried"} {
		if _, ok := doc.Paths[path]; ok {
			t.Errorf("%s documented though Authenticate refused it", path)
		}
	}
	for _, path := range []string{"/public", "/admin/open"} {
		if _, ok := doc.Paths[path]; !ok {
			t.Errorf("%s missing from the documentation", path)
		}
	}
}

// nilContextAuth returns the nil context a failing Authenticate is free to.
type nilContextAuth struct{}

func (nilContextAuth) Authenticate(ctx context.Context, r *http.Request, fn api.Function) (context.Context, error) {
	return nil, deniedError{}
}
func (nilContextAuth) Authorize(context.Context, *http.Request, api.Function, []reflect.Value) error {
	return nil
}
func (nilContextAuth) Redact(ctx context.Context, err error) error {
	_ = ctx.Value("request-id")
	return err
}

func TestRefusalKeepsARequestContext(t *testing.T) {
	type API struct {
		api.Specification
		Get func(context.Context) (string, error) `rest:"GET /get"`
	}
	h, err := rest.Handler(nilContextAuth{}, API{})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/get", "/documentation"} {
		if rec := serve(t, h, "GET", target, "", "Accept", "application/json"); rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s: %d", target, rec.Code)
		}
	}
}

// silentAuth redacts every error to nothing.
type silentAuth struct{}

func (silentAuth) Authenticate(ctx context.Context, r *http.Request, fn api.Function) (context.Context, error) {
	return ctx, nil
}
func (silentAuth) Authorize(context.Context, *http.Request, api.Function, []reflect.Value) error {
	return nil
}
func (silentAuth) Redact(context.Context, error) error { return nil }

func TestRedactingAnErrorToNilStillAnswers(t *testing.T) {
	type API struct {
		api.Specification
		Get func(context.Context) (string, error) `rest:"GET /get"`
	}
	h, err := rest.Handler(silentAuth{}, API{
		Get: func(context.Context) (string, error) { return "", errors.New("password=hunter2") },
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := serve(t, h, "GET", "/get", "")
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "hunter2") {
		t.Errorf("%d %q", rec.Code, rec.Body)
	}
}

// TestDocumentationAuthIsPerRequest drives signed in and anonymous callers at
// once. One request's Authenticate result must never decide another's response;
// under -race the shared variable that used to do so also fails the test.
func TestDocumentationAuthIsPerRequest(t *testing.T) {
	type API struct {
		api.Specification
		Get func(context.Context) (string, error) `rest:"GET /get"`
	}
	h, err := rest.Handler(tokenAuth{}, API{})
	if err != nil {
		t.Fatal(err)
	}
	var wrong, panics atomic.Int64
	var wg sync.WaitGroup
	for g := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5000 {
				func() {
					defer func() {
						if recover() != nil {
							panics.Add(1)
						}
					}()
					req := httptest.NewRequest("GET", "/documentation", nil)
					signedIn := g%2 == 0
					if signedIn {
						req.Header.Set("X-Token", "ok")
					}
					rec := httptest.NewRecorder()
					h.ServeHTTP(rec, req)
					if (rec.Code == http.StatusUnauthorized) == signedIn {
						wrong.Add(1)
					}
				}()
			}
		}()
	}
	wg.Wait()
	if wrong.Load() != 0 || panics.Load() != 0 {
		t.Errorf("%d answers for the wrong caller, %d panics", wrong.Load(), panics.Load())
	}
}

// A form on another site can have a browser send what it holds for this one,
// cookies and HTTP credentials alike, without asking CORS first.
func TestCrossSiteFormsAreRefused(t *testing.T) {
	type API struct {
		api.Specification
		Purge func(context.Context, string) error `rest:"POST /purge/{id=%v}"`
		Drop  func(context.Context, string) error `rest:"DELETE /purge/{id=%v}"`
	}
	var purged int
	impl := API{
		Purge: func(context.Context, string) error { purged++; return nil },
		Drop:  func(context.Context, string) error { purged++; return nil },
	}
	const form = "application/x-www-form-urlencoded"
	cases := []struct {
		name   string
		allow  string // what CORS answers, "" for an Auth without CORS
		method string
		ctype  string
		header []string
		ok     bool
	}{
		{"cross-site form", "", "POST", form, []string{"Origin", "https://evil.example", "Sec-Fetch-Site", "cross-site"}, false},
		{"sibling subdomain text", "", "POST", "text/plain", []string{"Sec-Fetch-Site", "same-site"}, false},
		{"cross-site, old browser", "", "POST", "multipart/form-data; boundary=x", []string{"Origin", "https://evil.example"}, false},
		{"cross-site with no body", "", "POST", "", []string{"Origin", "https://evil.example", "Sec-Fetch-Site", "cross-site"}, false},
		{"wildcard CORS form", "*", "POST", form, []string{"Origin", "https://evil.example", "Sec-Fetch-Site", "cross-site"}, true},
		{"wildcard CORS, no body", "*", "POST", "", []string{"Origin", "https://evil.example"}, true},
		{"other origin than CORS names", "https://app.example", "POST", form, []string{"Origin", "https://evil.example", "Sec-Fetch-Site", "cross-site"}, false},
		{"origin CORS names", "https://app.example", "POST", form, []string{"Origin", "https://app.example", "Sec-Fetch-Site", "cross-site"}, true},
		{"same origin form", "", "POST", form, []string{"Origin", "http://example.com", "Sec-Fetch-Site", "same-origin"}, true},
		{"no browser", "", "POST", form, nil, true},
		{"cross-site JSON, preflighted", "*", "POST", "application/json", []string{"Origin", "https://evil.example", "Sec-Fetch-Site", "cross-site"}, true},
		{"cross-site DELETE, preflighted", "*", "DELETE", "", []string{"Origin", "https://evil.example", "Sec-Fetch-Site", "cross-site"}, true},
	}
	for _, c := range cases {
		var auth api.Auth[*http.Request] = tokenAuth{}
		if c.allow != "" {
			auth = originAuth{allow: c.allow}
		}
		h, err := rest.Handler(auth, impl)
		if err != nil {
			t.Fatal(err)
		}
		purged = 0
		body := ""
		if c.ctype == "application/json" {
			body = `"1"`
		}
		rec := serve(t, h, c.method, "/purge/1", body, append([]string{"X-Token", "ok", "Content-Type", c.ctype}, c.header...)...)
		if ran := purged == 1; ran != c.ok || (rec.Code == http.StatusForbidden) == c.ok {
			t.Errorf("%s: %d, ran %v, want ran %v", c.name, rec.Code, ran, c.ok)
		}
	}

	var runs int
	type Docs struct {
		api.Specification
		api.Documentation
	}
	docs := &Docs{Documentation: func(ctx context.Context) (api.Examples, error) {
		return &testDocs{TestingFramework: api.TestingFramework{History: &fakeHistory{}}, runs: &runs}, nil
	}}
	h, err := rest.Handler(tokenAuth{}, docs)
	if err != nil {
		t.Fatal(err)
	}
	rec := serve(t, h, "POST", "/testruns/ExampleThing", "", "X-Token", "ok", "Content-Type", form,
		"Origin", "https://evil.example", "Sec-Fetch-Site", "cross-site")
	if rec.Code != http.StatusForbidden || runs != 0 {
		t.Errorf("a cross-site form ran a test: %d, %d runs", rec.Code, runs)
	}
	rec = serve(t, h, "POST", "/testruns/ExampleThing", "", "X-Token", "ok", "Content-Type", form,
		"Origin", "http://example.com", "Sec-Fetch-Site", "same-origin")
	if rec.Code != http.StatusSeeOther || runs != 1 {
		t.Errorf("the run button of the page itself: %d, %d runs", rec.Code, runs)
	}
}
