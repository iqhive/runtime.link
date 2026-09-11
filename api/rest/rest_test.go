package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
	"github.com/iqhive/runtime.link/xyz"
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
	expected := map[string]bool{
		"POST /:s":           true,
		"GET /":              true,
		"OPTIONS /:s":        true,
		"GET /:s":            true,
		"GET /documentation": true,
		"GET /assets/:file":  true,
	}
	for pattern, handler := range handlers {
		if !expected[pattern] {
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

// TestFileBodyRoundTrip proves an fs.File body parameter works end-to-end over
// REST: the client streams the file's raw bytes as the request body and the
// handler receives them as an fs.File (reading the bytes and the synthesized
// name/size from Stat). This backs customer document uploads (e.g. econnect
// ProcessUpload), where the browser POSTs raw image bytes to an fs.File body.
func TestFileBodyRoundTrip(t *testing.T) {
	type API struct {
		api.Specification

		Upload func(context.Context, string, fs.File) (string, error) `rest:"POST /upload/{id=%v} echo"`
	}
	impl := API{
		Upload: func(ctx context.Context, id string, file fs.File) (string, error) {
			if file == nil {
				return "", errors.New("nil file")
			}
			defer file.Close()
			data, err := io.ReadAll(file)
			if err != nil {
				return "", err
			}
			name := ""
			if info, err := file.Stat(); err == nil {
				name = info.Name()
			}
			return id + ":" + name + ":" + string(data), nil
		},
	}
	handler, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	client := api.Import[API](rest.API, server.URL, server.Client())

	fsys := fstest.MapFS{"licence.jpg": &fstest.MapFile{Data: []byte("front-of-licence")}}
	file, err := fsys.Open("licence.jpg")
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Upload(context.Background(), "tok123", file)
	if err != nil {
		t.Fatal(err)
	}
	// id echoes the path param; the body bytes stream through; the filename
	// rides on the Content-Disposition the client set from fs.File's Stat.
	if want := "tok123:licence.jpg:front-of-licence"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestTwoBodyArgsWithoutRules pins the behaviour of two body arguments
// without (rules) and without Go names: they are not mapped into a JSON
// object. A JSON object body is therefore not split across the two
// arguments.
func TestTwoBodyArgsWithoutRules(t *testing.T) {
	type API struct {
		api.Specification
		Pair func(context.Context, string, string) string `rest:"POST /pair"`
	}
	handler, err := rest.Handler(nil, API{
		Pair: func(ctx context.Context, a, b string) string {
			return a + ":" + b
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/pair", strings.NewReader(`{"left":"a","right":"b"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == 200 && rec.Body.String() == `"a:b"` {
		t.Fatal("unnamed two-body args should not map a JSON object by field name")
	}
}

type namedPairAPI struct {
	api.Specification
	Pair           func(ctx context.Context, left, right string) (string, error)     `rest:"POST /pair"`
	Echo           func(ctx context.Context, message string) (string, error)         `rest:"POST /echo"`
	Coords         func(ctx context.Context) (lat, lon float64, err error)           `rest:"GET /coords"`
	Override       func(ctx context.Context, left, right string) (string, error)     `rest:"POST /override (x,y)"`
	Triple         func(ctx context.Context, a, b, c string) (string, error)         `rest:"POST /triple"`
	PathAndBody    func(ctx context.Context, id, left, right string) (string, error) `rest:"POST /items/{id=%v}"`
	ResultOverride func(ctx context.Context) (lat, lon float64, err error)           `rest:"GET /xy x,y"`
	Partial        func(ctx context.Context, left string, _ string) (string, error)  `rest:"POST /partial"`
	Search         func(ctx context.Context, q, left, right string) (string, error)  `rest:"POST /search?q=%v"`
}

func registerNamedPairSource(t *testing.T) {
	t.Helper()
	api.RegisterSource[namedPairAPI](fstest.MapFS{
		"api.go": &fstest.MapFile{Data: []byte(`package rest_test

import (
	"context"
	"github.com/iqhive/runtime.link/api"
)

type namedPairAPI struct {
	api.Specification
	Pair           func(ctx context.Context, left, right string) (string, error) ` + "`rest:\"POST /pair\"`" + `
	Echo           func(ctx context.Context, message string) (string, error) ` + "`rest:\"POST /echo\"`" + `
	Coords         func(ctx context.Context) (lat, lon float64, err error) ` + "`rest:\"GET /coords\"`" + `
	Override       func(ctx context.Context, left, right string) (string, error) ` + "`rest:\"POST /override (x,y)\"`" + `
	Triple         func(ctx context.Context, a, b, c string) (string, error) ` + "`rest:\"POST /triple\"`" + `
	PathAndBody    func(ctx context.Context, id, left, right string) (string, error) ` + "`rest:\"POST /items/{id=%v}\"`" + `
	ResultOverride func(ctx context.Context) (lat, lon float64, err error) ` + "`rest:\"GET /xy x,y\"`" + `
	Partial        func(ctx context.Context, left string, _ string) (string, error) ` + "`rest:\"POST /partial\"`" + `
	Search         func(ctx context.Context, q, left, right string) (string, error) ` + "`rest:\"POST /search?q=%v\"`" + `
}
`)},
	})
}

func TestNamedBodyAndResultRules(t *testing.T) {
	registerNamedPairSource(t)
	impl := namedPairAPI{
		Pair: func(ctx context.Context, left, right string) (string, error) {
			return left + ":" + right, nil
		},
		Echo: func(ctx context.Context, message string) (string, error) {
			return message, nil
		},
		Coords: func(ctx context.Context) (lat, lon float64, err error) {
			return 12.2, 15.0, nil
		},
		Override: func(ctx context.Context, left, right string) (string, error) {
			return left + ":" + right, nil
		},
		Triple: func(ctx context.Context, a, b, c string) (string, error) {
			return a + b + c, nil
		},
		PathAndBody: func(ctx context.Context, id, left, right string) (string, error) {
			return id + ":" + left + ":" + right, nil
		},
		ResultOverride: func(ctx context.Context) (lat, lon float64, err error) {
			return 1, 2, nil
		},
		Partial: func(ctx context.Context, left string, _ string) (string, error) {
			return left, nil
		},
	}
	handler, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := api.Import[namedPairAPI](rest.API, server.URL, server.Client())

	got, err := client.Pair(context.Background(), "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if got != "a:b" {
		t.Fatalf("Pair = %q", got)
	}

	echo, err := client.Echo(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if echo != "hello" {
		t.Fatalf("Echo (single body) = %q", echo)
	}

	lat, lon, err := client.Coords(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if lat != 12.2 || lon != 15.0 {
		t.Fatalf("Coords = %v, %v", lat, lon)
	}

	over, err := client.Override(context.Background(), "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if over != "a:b" {
		t.Fatalf("Override = %q", over)
	}
	// Explicit (x,y) must win: a body keyed by Go names should not bind.
	req := httptest.NewRequest("POST", "/override", strings.NewReader(`{"left":"a","right":"b"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == 200 && strings.Contains(rec.Body.String(), "a:b") && !strings.Contains(rec.Body.String(), `:"`) {
		t.Fatal("explicit (x,y) rules should not accept Go names as keys")
	}
	req = httptest.NewRequest("POST", "/override", strings.NewReader(`{"x":"a","y":"b"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "a:b") {
		t.Fatalf("explicit (x,y) body: code=%d body=%q", rec.Code, rec.Body.String())
	}

	docReq := httptest.NewRequest("GET", "/documentation", nil)
	docReq.Header.Set("Accept", "application/json")
	docRec := httptest.NewRecorder()
	handler.ServeHTTP(docRec, docReq)
	if docRec.Code != 200 {
		t.Fatalf("openapi status %d: %s", docRec.Code, docRec.Body.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(docRec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	paths, _ := doc["paths"].(map[string]any)
	pair, _ := paths["/pair"].(map[string]any)
	post, _ := pair["post"].(map[string]any)
	body, _ := post["requestBody"].(map[string]any)
	content, _ := body["content"].(map[string]any)
	appJSON, _ := content["application/json"].(map[string]any)
	schema, _ := appJSON["schema"].(map[string]any)
	props, _ := schema["properties"].(map[string]any)
	if props["left"] == nil || props["right"] == nil {
		t.Fatalf("OpenAPI /pair properties = %#v", props)
	}

	formReq := httptest.NewRequest("POST", "/pair?method=POST", nil)
	formReq.Header.Set("Accept", "application/schema+json")
	formRec := httptest.NewRecorder()
	handler.ServeHTTP(formRec, formReq)
	var form map[string]any
	if err := json.Unmarshal(formRec.Body.Bytes(), &form); err != nil {
		t.Fatal(err)
	}
	formProps, _ := form["properties"].(map[string]any)
	if formProps["left"] == nil || formProps["right"] == nil {
		t.Fatalf("form schema properties = %#v", form)
	}

	triple, err := client.Triple(context.Background(), "x", "y", "z")
	if err != nil {
		t.Fatal(err)
	}
	if triple != "xyz" {
		t.Fatalf("Triple = %q", triple)
	}

	item, err := client.PathAndBody(context.Background(), "9", "L", "R")
	if err != nil {
		t.Fatal(err)
	}
	if item != "9:L:R" {
		t.Fatalf("PathAndBody = %q", item)
	}

	x, y, err := client.ResultOverride(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if x != 1 || y != 2 {
		t.Fatalf("ResultOverride = %v, %v", x, y)
	}
	xyReq := httptest.NewRequest("GET", "/xy", nil)
	xyRec := httptest.NewRecorder()
	handler.ServeHTTP(xyRec, xyReq)
	if xyRec.Code != 200 || !strings.Contains(xyRec.Body.String(), `"x"`) || !strings.Contains(xyRec.Body.String(), `"y"`) {
		t.Fatalf("explicit result names: code=%d body=%q", xyRec.Code, xyRec.Body.String())
	}
	if strings.Contains(xyRec.Body.String(), `"lat"`) {
		t.Fatalf("explicit result rules should win over Go names: %s", xyRec.Body.String())
	}

	// Partial names (blank ident) must not invent a JSON object mapping.
	partReq := httptest.NewRequest("POST", "/partial", strings.NewReader(`{"left":"a","_":"b"}`))
	partReq.Header.Set("Content-Type", "application/json")
	partRec := httptest.NewRecorder()
	handler.ServeHTTP(partRec, partReq)
	if partRec.Code == 200 && strings.TrimSpace(partRec.Body.String()) == `"a"` {
		t.Fatal("partial names should not map a JSON object")
	}

	sdkReq := httptest.NewRequest("GET", "/documentation", nil)
	sdkReq.Header.Set("Accept", "application/javascript")
	sdkRec := httptest.NewRecorder()
	handler.ServeHTTP(sdkRec, sdkReq)
	if sdkRec.Code != 200 {
		t.Fatalf("sdk status %d", sdkRec.Code)
	}
	if !strings.Contains(sdkRec.Body.String(), "/pair") || !strings.Contains(sdkRec.Body.String(), "POST") {
		t.Fatalf("JS SDK should include the named-body endpoint, got %q", sdkRec.Body.String())
	}

	coordsReq := httptest.NewRequest("GET", "/coords", nil)
	coordsRec := httptest.NewRecorder()
	handler.ServeHTTP(coordsRec, coordsReq)
	if coordsRec.Code != 200 || !strings.Contains(coordsRec.Body.String(), `"lat"`) || !strings.Contains(coordsRec.Body.String(), `"lon"`) {
		t.Fatalf("named results: code=%d body=%q", coordsRec.Code, coordsRec.Body.String())
	}
}

func TestUnnamedTwoResultsStillRequireRules(t *testing.T) {
	type API struct {
		api.Specification
		Split func(context.Context) (float64, float64, error) `rest:"GET /split"`
	}
	_, err := rest.Handler(nil, API{
		Split: func(context.Context) (float64, float64, error) { return 0, 0, nil },
	})
	if err == nil {
		t.Fatal("two unnamed results without rules should fail to load")
	}
}

func TestSingleNamedBodyIsRawJSON(t *testing.T) {
	registerNamedPairSource(t)
	impl := namedPairAPI{
		Echo: func(ctx context.Context, message string) (string, error) {
			return message, nil
		},
	}
	handler, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/echo", strings.NewReader(`"hello"`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "hello") {
		t.Fatalf("raw string body: code=%d body=%q", rec.Code, rec.Body.String())
	}
	obj := httptest.NewRequest("POST", "/echo", strings.NewReader(`{"message":"hello"}`))
	obj.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, obj)
	if rec.Code == 200 && strings.Contains(rec.Body.String(), "hello") && !strings.Contains(rec.Body.String(), "message") {
		t.Fatal("single named body should not object-map")
	}
}

func TestNamedQueryAndBody(t *testing.T) {
	registerNamedPairSource(t)
	impl := namedPairAPI{
		Search: func(ctx context.Context, q, left, right string) (string, error) {
			return q + ":" + left + ":" + right, nil
		},
	}
	handler, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/search?q=hello", strings.NewReader(`{"left":"a","right":"b"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "hello:a:b") {
		t.Fatalf("query+named body: code=%d body=%q", rec.Code, rec.Body.String())
	}

	server := httptest.NewServer(handler)
	defer server.Close()
	client := api.Import[namedPairAPI](rest.API, server.URL, server.Client())
	got, err := client.Search(context.Background(), "q", "L", "R")
	if err != nil {
		t.Fatal(err)
	}
	if got != "q:L:R" {
		t.Fatalf("Search = %q", got)
	}
}

func TestNamedResultsOpenAPI(t *testing.T) {
	registerNamedPairSource(t)
	impl := namedPairAPI{
		Coords: func(ctx context.Context) (lat, lon float64, err error) {
			return 1, 2, nil
		},
	}
	handler, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/documentation", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("openapi status %d: %s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if !strings.Contains(raw, "/coords") {
		t.Fatalf("missing /coords in OpenAPI: %s", raw)
	}
	if !strings.Contains(raw, `"lat"`) || !strings.Contains(raw, `"lon"`) {
		t.Fatalf("named results missing from OpenAPI: %s", raw)
	}
}
