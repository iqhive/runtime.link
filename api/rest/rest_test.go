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

	// single element with [] suffix
	req = httptest.NewRequest("POST", "/echo?strings[]=hello", nil)
	rec = httptest.NewRecorder()
	Handler.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != `[
	"hello"
]` {
		t.Fatal("unexpected body for single []: ", rec.Body.String())
	}

	// single element without [] suffix
	req = httptest.NewRequest("POST", "/echo?strings=hello", nil)
	rec = httptest.NewRecorder()
	Handler.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != `[
	"hello"
]` {
		t.Fatal("unexpected body for non-[]: ", rec.Body.String())
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

// TestNilPointerFieldInQuery tests that a struct with a pointer field can be
// used as a query parameter without panicking when the pointer is nil. This
// is a regression test for a panic in fieldByIndex when the client tried to
// Set an unaddressable reflect.Value.
func TestNilPointerFieldInQuery(t *testing.T) {
	type Inner struct {
		Value string `json:"value"`
	}
	type Request struct {
		Name  string `json:"name"`
		Inner *Inner `json:"inner,omitempty"`
	}
	type API struct {
		api.Specification

		Lookup func(context.Context, Request) (string, error) `rest:"GET /lookup?%v"`
	}
	impl := API{
		Lookup: func(ctx context.Context, req Request) (string, error) {
			return req.Name, nil
		},
	}
	handler, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	client := api.Import[API](rest.API, server.URL, server.Client())

	// Call with nil pointer field — this previously panicked.
	result, err := client.Lookup(context.Background(), Request{Name: "test", Inner: nil})
	if err != nil {
		t.Fatal(err)
	}
	if result != "test" {
		t.Fatalf("got %q, want %q", result, "test")
	}

	// Call with non-nil pointer field to confirm it still works.
	result, err = client.Lookup(context.Background(), Request{Name: "hello", Inner: &Inner{Value: "world"}})
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Fatalf("got %q, want %q", result, "hello")
	}
}

// statusError is an error type whose body doesn't say which status it came
// with: decodeError sets it.
type statusError struct {
	Code   int    `json:"-"`
	Reason string `json:"reason"`
}

func (e statusError) Error() string        { return e.Reason }
func (e statusError) StatusHTTP() int      { return e.Code }
func (e *statusError) SetStatusHTTP(c int) { e.Code = c }

func TestErrorsGetTheirStatus(t *testing.T) {
	server := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"reason":"no such thing"}`))
	})
	var API struct {
		api.Specification

		api.Register[error, statusError]

		Get    func(context.Context) error `rest:"GET /thing"`
		Delete func(context.Context) error `rest:"DELETE /thing"`
	}
	if err := rest.API.Link(api.StructureOf(&API), "http://example.com", TestTransport{server}.Client()); err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func(context.Context) error{"GET": API.Get, "DELETE": API.Delete} {
		err := call(context.Background())
		var got statusError
		if !errors.As(err, &got) || got.Code != http.StatusNotFound || got.Reason != "no such thing" {
			t.Errorf("%s: got %#v, want a statusError with the 404 and its reason", name, err)
		}
	}
}
