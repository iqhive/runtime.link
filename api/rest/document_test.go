package rest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

type described struct {
	api.Specification

	Get func(context.Context) (string, error) `rest:"GET /thing"`
}

// serversOf returns the servers of the OpenAPI document h answers a request
// with the given X-Base-Path with.
func serversOf(t *testing.T, h http.Handler, basePath string) []string {
	t.Helper()
	rec := serve(t, h, "GET", "/documentation", "", "Accept", "application/json", "X-Base-Path", basePath)
	var doc struct {
		Servers []struct {
			URL string `json:"url"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("X-Base-Path %q: %v", basePath, err)
	}
	var servers []string
	for _, server := range doc.Servers {
		servers = append(servers, server.URL)
	}
	return servers
}

// TestBasePathCannotNameAnotherHost checks that the servers of the OpenAPI
// document are only ever paths on the server that was asked. The header was
// checked as it was sent and answered decoded, so an escaped slash, or a
// backslash, after the first slash came out as the start of another host, which
// is how a browser, and so the page of the documentation, reads it.
func TestBasePathCannotNameAnotherHost(t *testing.T) {
	h, err := rest.Handler(nil, described{Get: say("")})
	if err != nil {
		t.Fatal(err)
	}
	withoutHeader := serversOf(t, h, "")
	for _, header := range []string{
		"/%2Fevil.example/x",
		"/%2fevil.example/x",
		`/\evil.example/x`,
		"/%5Cevil.example/x",
		"/v1, /%2f%2fevil.example",
		"//evil.example/x",
		"https://evil.example/x",
		"evil.example",
	} {
		if servers := serversOf(t, h, header); !reflect.DeepEqual(servers, withoutHeader) {
			t.Errorf("X-Base-Path %q: the document names the servers %q, want the header ignored", header, servers)
		}
	}
	// What is past the first segment stays escaped, so that it is still part
	// of a path to whoever decodes the document.
	for header, want := range map[string][]string{
		"/api/v1":            {"/api/v1"},
		"/v1, /v2":           {"/v1", "/v2"},
		"/":                  {"/"},
		"/my%20api":          {"/my%20api"},
		"/a/%2Fevil.example": {"/a/%2Fevil.example"},
		"/a%09/b":            {"/a%09/b"},
		"/%09/evil.example":  {"/%09/evil.example"},
	} {
		if servers := serversOf(t, h, header); !reflect.DeepEqual(servers, want) {
			t.Errorf("X-Base-Path %q: the document names the servers %q, want %q", header, servers, want)
		}
	}
}

type blob struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

type blobs struct {
	api.Specification

	Get    func(context.Context) ([]byte, error)          `rest:"GET /blob"`
	Put    func(context.Context, []byte) (string, error)  `rest:"PUT /blob"`
	Find   func(context.Context, []byte) (string, error)  `rest:"GET /blobs?data=%v"`
	Record func(context.Context, blob) (blob, error)      `rest:"POST /record"`
	Raw    func(context.Context) (json.RawMessage, error) `rest:"GET /raw"`
	Fixed  func(context.Context) ([4]byte, error)         `rest:"GET /fixed"`
	Bytes  func(context.Context, []byte) (string, error)  `rest:"GET /bytes/{data=%v}"`
}

// TestOpenAPIDescribesBytesAsTheyAreServed checks that the OpenAPI document
// describes a []byte in JSON as what encoding/json makes of it, a string of
// base64. It said an array of integers, which a client generated from the
// document fails to read a result as. In a query a []byte is still the list of
// numbers it is sent as.
func TestOpenAPIDescribesBytesAsTheyAreServed(t *testing.T) {
	h, err := rest.Handler(nil, blobs{
		Get:    func(context.Context) ([]byte, error) { return []byte("hi"), nil },
		Put:    func(_ context.Context, data []byte) (string, error) { return string(data), nil },
		Find:   func(_ context.Context, data []byte) (string, error) { return string(data), nil },
		Record: same[blob],
		Raw:    func(context.Context) (json.RawMessage, error) { return json.RawMessage(`{}`), nil },
		Fixed:  func(context.Context) ([4]byte, error) { return [4]byte{1, 2, 3, 4}, nil },
		Bytes:  func(_ context.Context, data []byte) (string, error) { return string(data), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	type schema struct {
		Type            any    `json:"type"`
		ContentEncoding string `json:"contentEncoding"`
		Reference       string `json:"$ref"`
	}
	type content map[string]struct {
		Schema schema `json:"schema"`
	}
	var doc struct {
		Paths map[string]map[string]struct {
			Parameters []struct {
				Name   string `json:"name"`
				Schema schema `json:"schema"`
			} `json:"parameters"`
			RequestBody struct {
				Content content `json:"content"`
			} `json:"requestBody"`
			Responses map[string]struct {
				Content content `json:"content"`
			} `json:"responses"`
		} `json:"paths"`
		Components struct {
			Schemas map[string]struct {
				Definitions map[string]struct {
					Properties map[string]schema `json:"properties"`
				} `json:"$defs"`
			} `json:"schemas"`
		} `json:"components"`
	}
	rec := serve(t, h, "GET", "/documentation", "", "Accept", "application/json")
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	// said is a schema's type and encoding, as text: a type may be a list.
	said := func(s schema) string { return fmt.Sprint(s.Type, " ", s.ContentEncoding) }
	const base64 = "string base64"
	if got := said(doc.Paths["/blob"]["get"].Responses["default"].Content["application/json"].Schema); got != base64 {
		t.Errorf("the result of GET /blob is documented as %q, want %q", got, base64)
	}
	if got := said(doc.Paths["/blob"]["put"].RequestBody.Content["application/json"].Schema); got != base64 {
		t.Errorf("the body of PUT /blob is documented as %q, want %q", got, base64)
	}
	field := doc.Components.Schemas["rest_test"].Definitions["blob"].Properties["data"]
	if said(field) != base64 {
		t.Errorf("a []byte field is documented as %q, want %q\n%s", said(field), base64, rec.Body)
	}
	// In a query a []byte is a value to each byte, and in a path the text of
	// the segment, which is what the document says of each.
	for path, want := range map[string]string{"/blobs": "array ", "/bytes/{data}": "string "} {
		parameters := doc.Paths[path]["get"].Parameters
		if len(parameters) != 1 || parameters[0].Name != "data" || said(parameters[0].Schema) != want {
			t.Errorf("the parameter of GET %s is documented as %+v, want %q", path, parameters, want)
		}
	}
	if taken := serve(t, h, "GET", "/blobs?data=104&data=105", "", "Accept", "text/plain"); taken.Body.String() != "hi" {
		t.Errorf("GET /blobs?data=104&data=105: %d %q", taken.Code, taken.Body)
	}
	if taken := serve(t, h, "GET", "/bytes/hi", "", "Accept", "text/plain"); taken.Body.String() != "hi" {
		t.Errorf("GET /bytes/hi: %d %q", taken.Code, taken.Body)
	}
	// What is no base64 in JSON is described as it was.
	for _, path := range []string{"/raw", "/fixed"} {
		if got := said(doc.Paths[path]["get"].Responses["default"].Content["application/json"].Schema); got != "array " {
			t.Errorf("the result of GET %s is documented as %q, want an array as before", path, got)
		}
	}

	// The document says what is served, and what is taken.
	if served := serve(t, h, "GET", "/blob", "", "Accept", "application/json"); strings.TrimSpace(served.Body.String()) != `"aGk="` {
		t.Errorf("GET /blob: %q", served.Body)
	}
	if taken := serve(t, h, "PUT", "/blob", `"aGk="`, "Content-Type", "application/json", "Accept", "text/plain"); taken.Body.String() != "hi" {
		t.Errorf("PUT /blob of base64: %d %q", taken.Code, taken.Body)
	}
}

// documented is the API of the package documentation, with a context for the
// test to see the request through.
type documented struct {
	api.Specification `www:"https://api.example.com/v1"`

	Echo func(ctx context.Context, message string) (string, error) `rest:"GET /echo?message=%v"`
}

// TestDocumentedExampleCallsItsHost checks what the package documentation says
// of its example: linked with no host, Echo calls the host of the www tag. The
// documentation named the host in a rest tag, which nothing reads.
func TestDocumentedExampleCallsItsHost(t *testing.T) {
	var requested string
	ctx := rest.Interceptor(context.Background(), func(req *http.Request, _ func(*http.Request) (*http.Response, error)) (*http.Response, error) {
		requested = req.URL.String()
		return respond(200, `"hi"`), nil
	})
	client := api.Import[documented](rest.API, "", nil)
	got, err := client.Echo(ctx, "hi")
	if err != nil || got != "hi" || requested != "https://api.example.com/v1/echo?message=hi" {
		t.Errorf("Echo = %q, %v; requested %q", got, err, requested)
	}
}
