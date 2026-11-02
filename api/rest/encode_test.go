package rest_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

type encodedThing struct {
	Name string `json:"name"`
}

func TestContentTypesWithoutADecoderAnswer415(t *testing.T) {
	type API struct {
		api.Specification
		Create func(context.Context, encodedThing) (string, error)   `rest:"POST /create"`
		Mapped func(context.Context, string, string) (string, error) `rest:"POST /mapped (a,b)"`
	}
	h, err := rest.Handler(nil, API{
		Create: func(_ context.Context, th encodedThing) (string, error) { return th.Name, nil },
		Mapped: func(_ context.Context, a, b string) (string, error) { return a + b, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, ct := range []string{"multipart/form-data; boundary=x", "application/json+schema"} {
		for _, target := range []string{"/create", "/mapped"} {
			rec := serve(t, h, "POST", target, `{"name":"x"}`, "Content-Type", ct)
			if rec.Code != http.StatusUnsupportedMediaType {
				t.Errorf("POST %s Content-Type %s: %d %q", target, ct, rec.Code, rec.Body)
			}
		}
	}
}

type hiddenUnexported struct {
	Name  string `json:"name"`
	cache []byte
}

type unexportedSlice struct {
	items []string
	Total int `json:"total"`
}

// nilSafeHeaders has a value receiver, so a nil pointer to it panics when called.
type nilSafeHeaders struct {
	Name string `json:"name"`
}

func (nilSafeHeaders) WriteHeadersHTTP(h http.Header) { h.Set("X-Name", "set") }
func (nilSafeHeaders) StatusHTTP() int                { return http.StatusAccepted }

func TestEncodersSurviveAnyResult(t *testing.T) {
	type API struct {
		api.Specification
		Hidden  func(context.Context) (hiddenUnexported, error)     `rest:"GET /hidden"`
		Priv    func(context.Context) (unexportedSlice, error)      `rest:"GET /priv"`
		Any     func(context.Context) (any, error)                  `rest:"GET /any"`
		Nil     func(context.Context) (*hiddenUnexported, error)    `rest:"GET /nil"`
		Headers func(context.Context) (*nilSafeHeaders, error)      `rest:"GET /headers"`
		Reader  func(context.Context) (io.ReadCloser, error)        `rest:"GET /reader"`
		Time    func(context.Context) (*time.Time, error)           `rest:"GET /time"`
		Slice   func(context.Context) ([]*hiddenUnexported, error)  `rest:"GET /slice"`
		Map     func(context.Context) (map[string]any, error)       `rest:"GET /map"`
		Channel func(context.Context) (func(func(int) bool), error) `rest:"GET /seq"`
		Named   func(context.Context) (string, any, error)          `rest:"GET /named a,b"`
	}
	h, err := rest.Handler(nil, API{
		Hidden:  func(context.Context) (hiddenUnexported, error) { return hiddenUnexported{Name: "a"}, nil },
		Priv:    func(context.Context) (unexportedSlice, error) { return unexportedSlice{Total: 1}, nil },
		Any:     func(context.Context) (any, error) { return nil, nil },
		Nil:     func(context.Context) (*hiddenUnexported, error) { return nil, nil },
		Headers: func(context.Context) (*nilSafeHeaders, error) { return nil, nil },
		Reader:  func(context.Context) (io.ReadCloser, error) { return nil, nil },
		Time:    func(context.Context) (*time.Time, error) { return nil, nil },
		Slice:   func(context.Context) ([]*hiddenUnexported, error) { return []*hiddenUnexported{nil}, nil },
		Map:     func(context.Context) (map[string]any, error) { return map[string]any{"a": nil}, nil },
		Channel: func(context.Context) (func(func(int) bool), error) { return nil, nil },
		Named:   func(context.Context) (string, any, error) { return "a", nil, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/hidden", "/priv", "/any", "/nil", "/headers", "/reader", "/time", "/slice", "/map", "/seq", "/named"} {
		for _, accept := range []string{"", "*/*", "application/json", "text/plain", "application/xml", "multipart/form-data", "application/octet-stream", "application/json+schema"} {
			serve(t, h, "GET", target, "", "Accept", accept)
		}
	}
	if rec := serve(t, h, "GET", "/any", "", "Accept", "application/json"); rec.Code != 200 || rec.Body.String() != "null" {
		t.Errorf("nil any as JSON: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "GET", "/any", "", "Accept", "text/plain"); rec.Code != 200 || rec.Body.Len() != 0 {
		t.Errorf("nil any as text: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "GET", "/priv", "", "Accept", "text/plain"); !strings.Contains(rec.Body.String(), `"total"`) {
		t.Errorf("unexported first field as text: %d %q", rec.Code, rec.Body)
	}
}

func TestMultipartEncodesSlicesField(t *testing.T) {
	type Page struct {
		Items []encodedThing `json:"items"`
		Tags  []string       `json:"tags"`
		Blob  []byte         `json:"blob"`
	}
	type API struct {
		api.Specification
		Get func(context.Context) (Page, error) `rest:"GET /page"`
	}
	h, err := rest.Handler(nil, API{
		Get: func(context.Context) (Page, error) {
			return Page{[]encodedThing{{"a"}, {"b"}}, []string{"x", "y"}, []byte("hi")}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := serve(t, h, "GET", "/page", "", "Accept", "multipart/form-data")
	for _, want := range []string{`name="items.name"`, `name="tags"`, "\r\nhi\r\n"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("no %q in %q", want, rec.Body)
		}
	}
	if strings.Contains(rec.Body.String(), "{a}") {
		t.Errorf("struct elements formatted with fmt: %q", rec.Body)
	}
}

// listing is a struct whose first field is the list text/plain shows.
type listing struct {
	Items []string `json:"items"`
	Total int      `json:"total"`
}

type renderedText struct{ value string }

func (r renderedText) MarshalText() ([]byte, error) { return []byte("<" + r.value + ">"), nil }

type stringered struct{ value string }

func (s stringered) String() string { return "s:" + s.value }

func TestPlainTextResults(t *testing.T) {
	type API struct {
		api.Specification
		Bytes    func(context.Context) ([]byte, error)         `rest:"GET /bytes"`
		Listing  func(context.Context) (listing, error)        `rest:"GET /listing"`
		Strings  func(context.Context) ([]string, error)       `rest:"GET /strings"`
		Texts    func(context.Context) ([]renderedText, error) `rest:"GET /texts"`
		Pointer  func(context.Context) (*renderedText, error)  `rest:"GET /pointer"`
		Stringer func(context.Context) (stringered, error)     `rest:"GET /stringer"`
		Duration func(context.Context) (time.Duration, error)  `rest:"GET /duration"`
		Mapped   func(context.Context) (int, []string, error)  `rest:"GET /mapped total,items"`
		Nil      func(context.Context) (*string, error)        `rest:"GET /nil"`
	}
	h, err := rest.Handler(nil, API{
		Nil:      func(context.Context) (*string, error) { return nil, nil },
		Bytes:    func(context.Context) ([]byte, error) { return []byte("hi"), nil },
		Listing:  func(context.Context) (listing, error) { return listing{[]string{"a", "b"}, 2}, nil },
		Strings:  func(context.Context) ([]string, error) { return []string{"a", "b"}, nil },
		Texts:    func(context.Context) ([]renderedText, error) { return []renderedText{{"a"}, {"b"}}, nil },
		Pointer:  func(context.Context) (*renderedText, error) { return &renderedText{"a"}, nil },
		Stringer: func(context.Context) (stringered, error) { return stringered{"a"}, nil },
		Duration: func(context.Context) (time.Duration, error) { return 90 * time.Minute, nil },
		Mapped:   func(context.Context) (int, []string, error) { return 2, []string{"a", "b"}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{
		// A list has a line per element, a []byte as much as any.
		"/bytes":    "104\n105\n",
		"/nil":      "<nil>",
		"/listing":  "a\nb\n",
		"/strings":  "a\nb\n",
		"/texts":    "<a>\n<b>\n",
		"/pointer":  "<a>",
		"/stringer": "s:a",
		"/duration": "1h30m0s",
		"/mapped":   "a\nb\n",
	} {
		rec := serve(t, h, "GET", target, "", "Accept", "text/plain")
		if rec.Body.String() != want {
			t.Errorf("GET %s: %d %q, want %q", target, rec.Code, rec.Body, want)
		}
	}
}

func TestPlainTextBodies(t *testing.T) {
	type API struct {
		api.Specification
		Echo  func(context.Context, string) (string, error) `rest:"POST(text/plain) /echo"`
		Count func(context.Context, int) (int, error)       `rest:"POST(text/plain) /count"`
		Bytes func(context.Context, []byte) (string, error) `rest:"POST(text/plain) /bytes"`
		Reply func(context.Context, string) (string, error) `rest:"POST /reply"`
	}
	impl := API{
		Echo:  func(_ context.Context, s string) (string, error) { return s, nil },
		Count: func(_ context.Context, n int) (int, error) { return n, nil },
		Bytes: func(_ context.Context, b []byte) (string, error) { return string(b), nil },
		Reply: func(_ context.Context, s string) (string, error) { return s, nil },
	}
	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "POST", "/echo", "hello big\nworld ", "Accept", "text/plain"); rec.Body.String() != "hello big\nworld " {
		t.Errorf("POST /echo: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "POST", "/count", "12\n"); rec.Body.String() != "12" {
		t.Errorf("POST /count: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "POST", "/count", "12abc"); rec.Code != http.StatusBadRequest {
		t.Errorf("POST /count 12abc: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "POST", "/bytes", "a b"); rec.Body.String() != "a b" {
		t.Errorf("POST /bytes: %d %q", rec.Code, rec.Body)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	if got, err := client.Echo(context.Background(), "hello big world"); err != nil || got != "hello big world" {
		t.Errorf("client.Echo: %q, %v", got, err)
	}
	third := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "first second third")
	}))
	defer third.Close()
	other := api.Import[API](rest.API, third.URL, third.Client())
	if got, err := other.Reply(context.Background(), "x"); err != nil || got != "first second third" {
		t.Errorf("text/plain response: %q, %v", got, err)
	}
}

func TestAcceptNegotiation(t *testing.T) {
	type API struct {
		api.Specification
		Thing func(context.Context) (encodedThing, error) `rest:"GET /thing"`
		Text  func(context.Context) (string, error)       `rest:"GET /text"`
	}
	h, err := rest.Handler(nil, API{
		Thing: func(context.Context) (encodedThing, error) { return encodedThing{"x"}, nil },
		Text:  func(context.Context) (string, error) { return "x", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		target, accept string
		status         int
		want           string
	}{
		{"/thing", "*/*;q=0.8", 200, "application/json"},
		{"/thing", "application/*", 200, "application/json"},
		{"/thing", "application/vnd.x+json, */*", 200, "application/json"},
		{"/thing", "text/*", 200, "text/plain"},
		{"/thing", "text/csv", 415, ""},
		{"/thing", "text/csv, image/*", 415, ""},
		{"/thing", "not a media type", 415, ""},
		{"/thing", "application/xml, application/json", 200, "application/xml"},
		{"/thing", "*/*;q=0.1, text/plain;q=0.2", 200, "text/plain"},
		{"/text", "*/*;q=0.8", 200, "text/plain"},
		{"/text", "text/*", 200, "text/plain"},
		{"/text", "application/*", 200, "application/json"},
	} {
		rec := serve(t, h, "GET", tc.target, "", "Accept", tc.accept)
		if rec.Code != tc.status {
			t.Errorf("GET %s Accept %q: %d %q, want %d", tc.target, tc.accept, rec.Code, rec.Body, tc.status)
			continue
		}
		if tc.status == 200 && rec.Header().Get("Content-Type") != tc.want {
			t.Errorf("GET %s Accept %q: Content-Type %q, want %q", tc.target, tc.accept, rec.Header().Get("Content-Type"), tc.want)
		}
	}
}

// TestUnacceptableAcceptIs415 pins the answer to a client that takes no type on
// offer: 415, with the types there are in Accept-Encoding, as it always was.
func TestUnacceptableAcceptIs415(t *testing.T) {
	type API struct {
		api.Specification
		Thing func(context.Context) (encodedThing, error) `rest:"POST /thing"`
	}
	h, err := rest.Handler(nil, API{
		Thing: func(context.Context) (encodedThing, error) { return encodedThing{"x"}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	const supported = "application/json, application/json+schema, application/octet-stream, application/xml, multipart/form-data, text/plain"
	for _, accept := range []string{"text/csv", "text/csv, image/*", "not a media type", "text/html"} {
		rec := serve(t, h, "POST", "/thing", "", "Accept", accept)
		if rec.Code != http.StatusUnsupportedMediaType || rec.Body.String() != "unsupported media type\n" || rec.Header().Get("Accept-Encoding") != supported {
			t.Errorf("Accept %q: %d %q Accept-Encoding %q", accept, rec.Code, rec.Body, rec.Header().Get("Accept-Encoding"))
		}
	}
}

// TestAcceptListOrderChoosesTheType pins how an Accept header naming types on
// offer is read: the first one listed wins, whatever its quality, as clients
// were answered before quality values were read at all.
func TestAcceptListOrderChoosesTheType(t *testing.T) {
	type API struct {
		api.Specification
		Thing func(context.Context) (encodedThing, error) `rest:"POST /thing"`
		Text  func(context.Context) (string, error)       `rest:"POST /text"`
	}
	h, err := rest.Handler(nil, API{
		Thing: func(context.Context) (encodedThing, error) { return encodedThing{"x"}, nil },
		Text:  func(context.Context) (string, error) { return "hello", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ target, accept, ctype, body string }{
		{"/thing", "application/xml;q=0.5, application/json", "application/xml", "<encodedThing><Name>x</Name></encodedThing>"},
		{"/thing", "application/json;q=0.5, application/xml", "application/json", "{\n\t\"name\": \"x\"\n}"},
		{"/thing", "application/json;q=0", "application/json", "{\n\t\"name\": \"x\"\n}"},
		{"/text", "text/plain;q=0.5, application/json", "text/plain", "hello"},
		{"/text", "application/json;q=0, text/plain", "application/json", `"hello"`},
	} {
		rec := serve(t, h, "POST", tc.target, "", "Accept", tc.accept)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != tc.ctype || rec.Body.String() != tc.body {
			t.Errorf("POST %s Accept %q: %d ct=%q %q, want %s %q", tc.target, tc.accept, rec.Code, rec.Header().Get("Content-Type"), rec.Body, tc.ctype, tc.body)
		}
	}
}

// pointerStringer prints itself from a pointer receiver.
type pointerStringer struct{ value string }

func (s *pointerStringer) String() string { return "p:" + s.value }

// TestDefaultContentTypeFollowsTheResultType pins the type a client that takes
// anything is answered with: text/plain for a result that is not a container by
// its declared type and has a text form, nil included, as it always was. A value
// that fmt could only dump the fields of is JSON.
func TestDefaultContentTypeFollowsTheResultType(t *testing.T) {
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	type API struct {
		api.Specification
		Time      func(context.Context) (*time.Time, error)       `rest:"POST /time"`
		Text      func(context.Context) (*renderedText, error)    `rest:"POST /text"`
		Stringer  func(context.Context) (*stringered, error)      `rest:"POST /stringer"`
		PStringer func(context.Context) (*pointerStringer, error) `rest:"POST /pstringer"`
		Any       func(context.Context) (any, error)              `rest:"POST /any"`
		AnyStruct func(context.Context) (any, error)              `rest:"POST /anystruct"`
		Struct    func(context.Context) (*encodedThing, error)    `rest:"POST /struct"`
		Nil       func(context.Context) (*encodedThing, error)    `rest:"POST /nil"`
		Page      func(context.Context) (string, error)           `rest:"POST(text/html) /page"`
		PageBytes func(context.Context) ([]byte, error)           `rest:"POST(text/html) /pagebytes"`
	}
	h, err := rest.Handler(nil, API{
		Time:      func(context.Context) (*time.Time, error) { return &when, nil },
		Text:      func(context.Context) (*renderedText, error) { return &renderedText{"t"}, nil },
		Stringer:  func(context.Context) (*stringered, error) { return &stringered{"s"}, nil },
		PStringer: func(context.Context) (*pointerStringer, error) { return &pointerStringer{"s"}, nil },
		Any:       func(context.Context) (any, error) { return []string{"a", "b"}, nil },
		AnyStruct: func(context.Context) (any, error) { return encodedThing{"x"}, nil },
		Struct:    func(context.Context) (*encodedThing, error) { return &encodedThing{"x"}, nil },
		Nil:       func(context.Context) (*encodedThing, error) { return nil, nil },
		Page:      func(context.Context) (string, error) { return "<h1>hi</h1>", nil },
		PageBytes: func(context.Context) ([]byte, error) { return []byte("hi"), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, accept := range []string{"", "*/*"} {
		for _, tc := range []struct{ target, ctype, body string }{
			{"/time", "text/plain", "2026-01-02T03:04:05Z"},
			{"/text", "text/plain", "<t>"},
			{"/stringer", "text/plain", "s:s"},
			{"/pstringer", "text/plain", "p:s"},
			{"/any", "text/plain", "a\nb\n"},
			{"/anystruct", "application/json", "{\n\t\"name\": \"x\"\n}"},
			{"/struct", "application/json", "{\n\t\"name\": \"x\"\n}"},
			{"/nil", "text/plain", "<nil>"},
			{"/page", "text/plain", "<h1>hi</h1>"},
			{"/pagebytes", "application/json", `"aGk="`},
		} {
			var rec *httptest.ResponseRecorder
			switch accept {
			case "":
				rec = serve(t, h, "POST", tc.target, "")
			default:
				rec = serve(t, h, "POST", tc.target, "", "Accept", accept)
			}
			if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != tc.ctype || rec.Body.String() != tc.body {
				t.Errorf("POST %s Accept %q: %d ct=%q %q, want %s %q", tc.target, accept, rec.Code, rec.Header().Get("Content-Type"), rec.Body, tc.ctype, tc.body)
			}
		}
	}
}

func TestDeclaredContentTypeWithoutAnEncoder(t *testing.T) {
	type API struct {
		api.Specification
		Page func(context.Context) (string, error) `rest:"GET(text/html) /page"`
		Png  func(context.Context) ([]byte, error) `rest:"GET(image/png) /png"`
	}
	h, err := rest.Handler(nil, API{
		Page: func(context.Context) (string, error) { return "<h1>hi</h1>", nil },
		Png:  func(context.Context) ([]byte, error) { return []byte("\x89PNG"), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, accept := range []string{"text/html", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", "text/*"} {
		rec := serve(t, h, "GET", "/page", "", "Accept", accept)
		if ct := rec.Header().Get("Content-Type"); rec.Code != 200 || !strings.HasPrefix(ct, "text/html") || rec.Body.String() != "<h1>hi</h1>" {
			t.Errorf("Accept %q: %d ct=%q %q", accept, rec.Code, ct, rec.Body)
		}
	}
	// A client that takes anything is answered as it always was, by the
	// result's type and not the declared one.
	for _, accept := range []string{"", "*/*"} {
		rec := serve(t, h, "GET", "/page", "", "Accept", accept)
		if ct := rec.Header().Get("Content-Type"); rec.Code != 200 || ct != "text/plain" || rec.Body.String() != "<h1>hi</h1>" {
			t.Errorf("Accept %q: %d ct=%q %q", accept, rec.Code, ct, rec.Body)
		}
	}
	if rec := serve(t, h, "GET", "/page", "", "Accept", "application/json"); rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("an explicit Accept still wins: %d ct=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	rec := serve(t, h, "GET", "/png", "", "Accept", "image/png")
	if rec.Header().Get("Content-Type") != "image/png" || rec.Body.String() != "\x89PNG" {
		t.Errorf("GET /png: %d ct=%q %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}
}

type createdThing struct {
	ID string `json:"id"`
}

func (createdThing) StatusHTTP() int { return http.StatusCreated }

type zeroStatus struct {
	ID     string `json:"id"`
	Status int    `json:"-"`
}

func (s zeroStatus) StatusHTTP() int { return s.Status }

// customType sets its own Content-Type from a value's headers.
type customType struct {
	ID string `json:"id"`
}

func (customType) WriteHeadersHTTP(h http.Header) {
	h.Set("Content-Type", "application/vnd.custom+json")
}

func TestResultStatusAndHeaders(t *testing.T) {
	type API struct {
		api.Specification
		Create func(context.Context) (createdThing, error) `rest:"POST /create"`
		Zero   func(context.Context) (zeroStatus, error)   `rest:"POST /zero"`
		Bad    func(context.Context) (zeroStatus, error)   `rest:"POST /bad"`
		Custom func(context.Context) (customType, error)   `rest:"POST /custom"`
	}
	h, err := rest.Handler(nil, API{
		Create: func(context.Context) (createdThing, error) { return createdThing{"1"}, nil },
		Zero:   func(context.Context) (zeroStatus, error) { return zeroStatus{ID: "1"}, nil },
		Bad:    func(context.Context) (zeroStatus, error) { return zeroStatus{ID: "1", Status: 7}, nil },
		Custom: func(context.Context) (customType, error) { return customType{"1"}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/create", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated || resp.Header.Get("Content-Type") != "application/json" || resp.ContentLength != int64(len(body)) {
		t.Errorf("POST /create: %d ct=%q length=%d body=%q", resp.StatusCode, resp.Header.Get("Content-Type"), resp.ContentLength, body)
	}
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	if got, err := client.Create(context.Background()); err != nil || got.ID != "1" {
		t.Errorf("client.Create: %+v, %v", got, err)
	}
	for _, target := range []string{"/zero", "/bad"} {
		if rec := serve(t, h, "POST", target, "", "Accept", "application/json"); rec.Code != 200 {
			t.Errorf("POST %s: %d %q", target, rec.Code, rec.Body)
		}
	}
	if rec := serve(t, h, "POST", "/custom", "", "Accept", "application/json"); rec.Header().Get("Content-Type") != "application/vnd.custom+json" {
		t.Errorf("POST /custom: Content-Type %q", rec.Header().Get("Content-Type"))
	}
}

// closingFile is a WriterTo and a ReadCloser, as *os.File is.
type closingFile struct {
	strings.Reader
	closed *atomic.Bool
}

func (f *closingFile) Close() error { f.closed.Store(true); return nil }

func TestStreamingResultsAreClosed(t *testing.T) {
	var closed atomic.Bool
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/f", []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	var opened *os.File
	type API struct {
		api.Specification
		File   func(context.Context) (io.ReadCloser, error) `rest:"GET(text/plain) /file"`
		OSFile func(context.Context) (io.ReadCloser, error) `rest:"GET(text/plain) /osfile"`
	}
	h, err := rest.Handler(nil, API{
		File: func(context.Context) (io.ReadCloser, error) {
			f := &closingFile{closed: &closed}
			f.Reader = *strings.NewReader("hello")
			return f, nil
		},
		OSFile: func(context.Context) (io.ReadCloser, error) {
			f, err := os.Open(dir + "/f")
			opened = f
			return f, err
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "GET", "/file", ""); rec.Body.String() != "hello" || !closed.Load() {
		t.Errorf("GET /file: %q closed=%v", rec.Body, closed.Load())
	}
	if rec := serve(t, h, "GET", "/osfile", ""); rec.Body.String() != "hello" {
		t.Errorf("GET /osfile: %q", rec.Body)
	}
	if _, err := opened.Stat(); err == nil {
		t.Errorf("*os.File result still open after the response")
		opened.Close()
	}
}

// pngPicture sets the Content-Type a declared reader would otherwise replace.
type pngPicture struct {
	io.ReadCloser
}

func (pngPicture) WriteHeadersHTTP(header http.Header) { header.Set("Content-Type", "image/png") }

func TestReaderResultKeepsItsContentType(t *testing.T) {
	type API struct {
		api.Specification
		Picture func(context.Context) (pngPicture, error) `rest:"GET /picture"`
	}
	h, err := rest.Handler(nil, API{
		Picture: func(context.Context) (pngPicture, error) {
			return pngPicture{io.NopCloser(strings.NewReader("\x89PNG"))}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "GET", "/picture", ""); rec.Header().Get("Content-Type") != "image/png" {
		t.Errorf("Content-Type %q", rec.Header().Get("Content-Type"))
	}
}

// TestOnlyTheReturnedErrorChoosesItsContentType pins that the error a function
// returns picks the body's content type, not one it wraps: a wrapped error is
// answered with the message of the error that wraps it, as it always was.
func TestOnlyTheReturnedErrorChoosesItsContentType(t *testing.T) {
	type API struct {
		api.Specification
		Bare func(context.Context) error `rest:"POST /bare"`
		Join func(context.Context) error `rest:"POST /join"`
		Wrap func(context.Context) error `rest:"POST /wrap"`
	}
	h, err := rest.Handler(nil, API{
		Bare: func(context.Context) error { return throttled{Reason: "too many"} },
		Join: func(context.Context) error { return errors.Join(throttled{Reason: "too many"}) },
		Wrap: func(context.Context) error { return fmt.Errorf("while checking: %w", throttled{Reason: "too many"}) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "POST", "/bare", ""); rec.Code != http.StatusTooManyRequests || rec.Header().Get("Content-Type") != "application/json" || !strings.Contains(rec.Body.String(), "too many") {
		t.Errorf("POST /bare: %d ct=%q %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}
	for target, want := range map[string]string{"/join": "throttled\n", "/wrap": "while checking: throttled\n"} {
		rec := serve(t, h, "POST", target, "")
		if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" || rec.Body.String() != want {
			t.Errorf("POST %s: %d ct=%q %q, want %q", target, rec.Code, rec.Header().Get("Content-Type"), rec.Body, want)
		}
	}
}

const decodedBodyLimit = 32 << 20

// oversized is a JSON string body one byte past the limit, without the memory
// a literal one would cost.
func oversized(open, close string) io.Reader {
	return io.MultiReader(strings.NewReader(open), io.LimitReader(repeatedByte('a'), decodedBodyLimit+1), strings.NewReader(close))
}

type repeatedByte byte

func (r repeatedByte) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

func TestDecodedBodiesAreLimited(t *testing.T) {
	type API struct {
		api.Specification
		JSON  func(context.Context, string) (int, error)         `rest:"POST /json"`
		Text  func(context.Context, string) (int, error)         `rest:"POST(text/plain) /text"`
		XML   func(context.Context, encodedThing) (int, error)   `rest:"POST(application/xml) /xml"`
		Pair  func(context.Context, string, string) (int, error) `rest:"POST /pair (a,b)"`
		Array func(context.Context, string, string) (int, error) `rest:"POST /array"`
		Small func(context.Context, string) (int, error)         `rest:"POST /small"`
	}
	size := func(_ context.Context, s string) (int, error) { return len(s), nil }
	h, err := rest.Handler(nil, API{
		JSON: size, Text: size, Small: size,
		XML:   func(_ context.Context, th encodedThing) (int, error) { return len(th.Name), nil },
		Pair:  func(_ context.Context, a, b string) (int, error) { return len(a) + len(b), nil },
		Array: func(_ context.Context, a, b string) (int, error) { return len(a) + len(b), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		target, open, close, ctype string
	}{
		{"/json", `"`, `"`, "application/json"},
		{"/text", "", "", "text/plain"},
		{"/xml", "<a><Name>", "</Name></a>", "application/xml"},
		{"/pair", `{"a":"`, `"}`, "application/json"},
		{"/array", `["`, `","b"]`, "application/json"},
	} {
		req := httptest.NewRequest("POST", tc.target, oversized(tc.open, tc.close))
		req.Header.Set("Content-Type", tc.ctype)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("POST %s over the limit: %d %.100q", tc.target, rec.Code, rec.Body)
		}
	}
	if rec := serve(t, h, "POST", "/small", `"under"`); rec.Code != 200 {
		t.Errorf("POST /small: %d %q", rec.Code, rec.Body)
	}
}

func TestStreamedBodiesAreNotLimited(t *testing.T) {
	type API struct {
		api.Specification
		Reader func(context.Context, io.Reader) (int64, error)     `rest:"POST /reader"`
		Closer func(context.Context, io.ReadCloser) (int64, error) `rest:"POST /closer"`
	}
	drain := func(_ context.Context, r io.Reader) (int64, error) { return io.Copy(io.Discard, r) }
	h, err := rest.Handler(nil, API{
		Reader: drain,
		Closer: func(ctx context.Context, r io.ReadCloser) (int64, error) { defer r.Close(); return drain(ctx, r) },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/reader", "/closer"} {
		req := httptest.NewRequest("POST", target, io.LimitReader(repeatedByte('a'), decodedBodyLimit+1))
		req.Header.Set("Content-Type", "application/octet-stream")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if want := fmt.Sprint(decodedBodyLimit + 1); rec.Code != 200 || rec.Body.String() != want {
			t.Errorf("POST %s: %d %.100q, want %s", target, rec.Code, rec.Body, want)
		}
	}
}

func TestExampleStepDepthsMayJumpAround(t *testing.T) {
	impl := &stepExamples{steps: []api.Step{
		{Depth: 1, Call: &api.Function{Name: "A"}},
		{Depth: 9, Call: &api.Function{Name: "B"}},
		{Depth: 4, Call: &api.Function{Name: "C"}},
		{Depth: 0, Call: &api.Function{Name: "D"}},
	}}
	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "GET", "/examples/Steps", ""); rec.Code != 200 {
		t.Errorf("GET /examples/Steps: %d", rec.Code)
	}
}

// stepExamples documents one example made of the given steps.
type stepExamples struct {
	api.Specification
	steps []api.Step
}

func (e *stepExamples) Examples(context.Context) (map[string][]string, error) {
	return map[string][]string{"all": {"Steps"}}, nil
}
func (e *stepExamples) Example(_ context.Context, name string) (api.Example, bool) {
	return api.Example{Title: name, Steps: e.steps}, true
}

// emptyPage answers for itself when there is none, from pointer receivers.
type emptyPage struct {
	Items []string `json:"items"`
}

func (p *emptyPage) StatusHTTP() int {
	if p == nil {
		return http.StatusNotFound
	}
	return http.StatusOK
}

func (p *emptyPage) WriteHeadersHTTP(h http.Header) {
	if p == nil {
		h.Set("X-Empty", "true")
	}
}

// TestNilResultAnswersThroughPointerMethods pins that a nil result whose status
// and header methods are on the pointer receiver is still asked for them, as it
// always was: only a method on the value receiver cannot be called on nil.
func TestNilResultAnswersThroughPointerMethods(t *testing.T) {
	type API struct {
		api.Specification
		Page func(context.Context) (*emptyPage, error) `rest:"GET /page"`
	}
	h, err := rest.Handler(nil, API{
		Page: func(context.Context) (*emptyPage, error) { return nil, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := serve(t, h, "GET", "/page", "", "Accept", "application/json")
	if rec.Code != http.StatusNotFound || rec.Header().Get("X-Empty") != "true" || rec.Body.String() != "null" {
		t.Errorf("GET /page: %d X-Empty=%q %q", rec.Code, rec.Header().Get("X-Empty"), rec.Body)
	}
}
