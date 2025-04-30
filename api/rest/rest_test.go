package rest_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"runtime.link/api"
	"runtime.link/api/rest"
	"runtime.link/xyz"
)

type TestTransport struct {
	server http.Handler
}

func (t TestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	t.server.ServeHTTP(rec, req)
	return rec.Result(), nil
}

func (t TestTransport) Client() *http.Client {
	return &http.Client{Transport: t}
}

func TestErrors(t *testing.T) {
	type Error api.Error[struct {
		Internal xyz.Case[Error, error] `http:"500"
			internal server error`
		AccessDenied Error `http:"403"
			access denied`
	}]
	var Errors = xyz.AccessorFor(Error.Values)
	var API struct {
		api.Specification

		api.Register[error, Error]

		DoSomething func(context.Context) error `rest:"POST /"`
	}
	API.DoSomething = func(ctx context.Context) error {
		return Errors.AccessDenied
	}

	handler, err := rest.Handler(nil, &API)
	if err != nil {
		t.Fatal(err)
	}

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest("POST", "/", nil))

	if resp.Code != 403 {
		t.Errorf("got %v, want %v", resp.Code, 403)
	}
	if resp.Body.String() != "access denied\n" {
		t.Errorf("got %q, want %q", resp.Body.String(), "access denied\n")
	}
}

func TestParams(t *testing.T) {
	type API struct {
		api.Specification

		Echo func(context.Context, string, bool) string `rest:"POST /{s=%v}?reverse=%v"`
	}
	var impl = API{
		Echo: func(ctx context.Context, s string, reverse bool) string {
			if reverse {
				r := []rune(s)
				for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
					r[i], r[j] = r[j], r[i]
				}
				s = string(r)
			}
			return s
		},
	}
	var Handler, err = rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	handlers, err := rest.Handlers(nil, impl, ":%s", "*")
	if err != nil {
		t.Fatal(err)
	}
	for pattern, handler := range handlers {
		if pattern != "POST /:s" && pattern != "GET /" && pattern != "OPTIONS /:s" && pattern != "GET /:s" {
			t.Fatalf("unexpected pattern: %s", pattern)
		}
		if handler == nil {
			t.Fatalf("unexpected handler: %s", pattern)
		}
	}
	req := httptest.NewRequest("POST", "/foo", nil)
	rec := httptest.NewRecorder()
	Handler.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != `foo` {
		t.Fatal("unexpected body: ", rec.Body.String())
	}

	req = httptest.NewRequest("POST", "/foo?reverse=true", nil)
	rec = httptest.NewRecorder()
	Handler.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != `oof` {
		t.Fatal("unexpected body: ", rec.Body.String())
	}
}

func TestSliceParams(t *testing.T) {
	type API struct {
		api.Specification

		Echo func(context.Context, []string) []string `rest:"POST /echo?strings=%v"`
	}
	var Handler, err = rest.Handler(nil, API{
		Echo: func(ctx context.Context, s []string) []string {
			return s
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/echo?strings[]=hello&strings[]=world", nil)
	rec := httptest.NewRecorder()
	Handler.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != `[
	"hello",
	"world"
]` {
		t.Fatal("unexpected body: ", rec.Body.String())
	}
}

func TestSpecificity(t *testing.T) {
	type API struct {
		api.Specification

		DoSomethingGeneric  func(string) string `rest:"POST /do-something/{generic=%v}"`
		DoSomethingSpecific func() string       `rest:"POST /do-something/specific"`
	}
	var Handler, err = rest.Handler(nil, API{
		DoSomethingGeneric:  func(s string) string { return "generic[" + s + "]" },
		DoSomethingSpecific: func() string { return "specific" },
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/do-something/specific", nil)
	rec := httptest.NewRecorder()
	Handler.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != `specific` {
		t.Fatal("unexpected body: ", rec.Body.String())
	}

	req = httptest.NewRequest("POST", "/do-something/else", nil)
	rec = httptest.NewRecorder()
	Handler.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != `generic[else]` {
		t.Fatal("unexpected body: ", rec.Body.String())
	}
}

func TestAliases(t *testing.T) {
	type API struct {
		api.Specification

		DoSomething     func(s string) string `rest:"POST /do-something/{s=%v}"`
		DoSomethingElse func(s string) string `rest:"POST /do-something/{b=%v}/else"`
	}
	var Handler, err = rest.Handler(nil, API{
		DoSomething:     func(s string) string { return "DoSomething[" + s + "]" },
		DoSomethingElse: func(s string) string { return "DoSomethingElse[" + s + "]" },
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/do-something/foo", nil)
	rec := httptest.NewRecorder()
	Handler.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != `DoSomething[foo]` {
		t.Fatal("unexpected body: ", rec.Body.String())
	}

	req = httptest.NewRequest("POST", "/do-something/bar/else", nil)
	rec = httptest.NewRecorder()
	Handler.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != `DoSomethingElse[bar]` {
		t.Fatal("unexpected body: ", rec.Body.String())
	}
}

// TestFallback demonstrates how APIs can be composed.
func TestFallback(t *testing.T) {

	type API1 struct {
		api.Specification

		DoSomething func() string `rest:"POST /do-something"`
	}

	type API2 struct {
		api.Specification

		DoSomethingElse func() string `rest:"POST /do-something-else"`
	}

	var Handler1, _ = rest.Handler(nil, API1{
		DoSomething: func() string { return "DoSomething" },
	})
	var Handler2, _ = rest.Handler(nil, API2{
		DoSomethingElse: func() string { return "DoSomethingElse" },
	})

	router := (Handler1.(interface {
		http.Handler

		SetNotFoundHandler(http.Handler)
	}))
	router.SetNotFoundHandler(Handler2)

	req := httptest.NewRequest("POST", "/do-something-else", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != `DoSomethingElse` {
		t.Fatal("unexpected body: ", rec.Body.String())
	}

	req = httptest.NewRequest("POST", "/do-something", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != `DoSomething` {
		t.Fatal("unexpected body")
	}

	req = httptest.NewRequest("POST", "/1234", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatal("unexpected body")
	}
}

func TestExpansion(t *testing.T) {
	type Query struct {
		Param1 string `json:"param1"`
		Param2 bool   `json:"param2"`
	}
	type API struct {
		api.Specification

		GetSomething func(context.Context, Query) error `rest:"GET /something?%v"`
	}
	handler, err := rest.Handler(nil, API{
		GetSomething: func(ctx context.Context, q Query) error {
			if q.Param1 != "foo" || !q.Param2 {
				return errors.New("unexpected query")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/something?param1=foo&param2=true", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatal("unexpected status: ", rec.Code)
	}
}

func TestMapping(t *testing.T) {
	type API struct {
		api.Specification

		GetSomething func(context.Context, string, int64) (string, int64, error) `rest:"POST /something (a,b) a,b"`
	}
	handler, err := rest.Handler(nil, API{
		GetSomething: func(ctx context.Context, a string, b int64) (string, int64, error) {
			return a, b, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := TestTransport{server: handler}

	req := httptest.NewRequest("POST", "/something", strings.NewReader(`{"a":"foo","b":1234}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatal("unexpected status: ", rec.Code)
	}
	if rec.Body.String() != `{
	"a": "foo",
	"b": 1234
}` {
		t.Fatal("unexpected body: ", rec.Body.String())
	}

	client := api.Import[API](rest.API, "http://example.com", server.Client())
	a, b, err := client.GetSomething(context.Background(), "foo", 1234)
	if err != nil {
		t.Fatal(err)
	}
	if a != "foo" || b != 1234 {
		t.Fatal("unexpected result: ", a, b)
	}
}
