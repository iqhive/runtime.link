package rest_test

import (
	"context"
	"errors"
	"io"
	"iter"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

// pagedResult carries a value in a header, in both directions.
type pagedResult struct {
	Items []string `json:"items"`
	Total string   `json:"-"`
}

func (p *pagedResult) ReadHeadersHTTP(h http.Header) { p.Total = h.Get("X-Total") }
func (p pagedResult) WriteHeadersHTTP(h http.Header) { h.Set("X-Total", p.Total) }

func TestClientResultsReadHeaders(t *testing.T) {
	type API struct {
		api.Specification
		Pointer func(context.Context) (*pagedResult, error) `rest:"GET /pointer"`
		Value   func(context.Context) (pagedResult, error)  `rest:"GET /value"`
	}
	h, err := rest.Handler(nil, API{
		Pointer: func(context.Context) (*pagedResult, error) { return &pagedResult{[]string{"a"}, "7"}, nil },
		Value:   func(context.Context) (pagedResult, error) { return pagedResult{[]string{"a"}, "7"}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	if p, err := client.Pointer(context.Background()); err != nil || p == nil || len(p.Items) != 1 || p.Total != "7" {
		t.Errorf("pointer result: %+v, %v", p, err)
	}
	if v, err := client.Value(context.Background()); err != nil || len(v.Items) != 1 || v.Total != "7" {
		t.Errorf("value result: %+v, %v", v, err)
	}
}

func TestClientReturnsOpenBodies(t *testing.T) {
	type API struct {
		api.Specification
		Reader func(context.Context) (io.Reader, error)     `rest:"GET /r" mime:"text/plain"`
		Writer func(context.Context) (io.WriterTo, error)   `rest:"GET /w" mime:"text/plain"`
		Closer func(context.Context) (io.ReadCloser, error) `rest:"GET /c" mime:"text/plain"`
	}
	body := strings.Repeat("x", 1<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, body)
	}))
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	rc, err := client.Closer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil || len(got) != len(body) {
		t.Errorf("io.ReadCloser: read %d of %d, %v", len(got), len(body), err)
	}
	r, err := client.Reader(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got, err = io.ReadAll(r); err != nil || len(got) != len(body) {
		t.Errorf("io.Reader: read %d of %d, %v", len(got), len(body), err)
	}
	wt, err := client.Writer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var sink strings.Builder
	if n, err := wt.WriteTo(&sink); err != nil || int(n) != len(body) {
		t.Errorf("io.WriterTo: wrote %d of %d, %v", n, len(body), err)
	}
}

func TestClientMappedPointerResult(t *testing.T) {
	type API struct {
		api.Specification
		Two func(context.Context) (*encodedThing, int, error) `rest:"GET /two a,b"`
	}
	h, err := rest.Handler(nil, API{
		Two: func(context.Context) (*encodedThing, int, error) { return &encodedThing{"x"}, 2, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	defer func() {
		if p := recover(); p != nil {
			t.Errorf("mapped pointer result panicked: %v", p)
		}
	}()
	if a, b, err := client.Two(context.Background()); err != nil || a == nil || a.Name != "x" || b != 2 {
		t.Errorf("client.Two: %+v %d %v", a, b, err)
	}
}

func TestClientNilResultStaysNil(t *testing.T) {
	type API struct {
		api.Specification
		Get func(context.Context) (*encodedThing, error) `rest:"GET /get"`
	}
	h, err := rest.Handler(nil, API{Get: func(context.Context) (*encodedThing, error) { return nil, nil }})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	if got, err := client.Get(context.Background()); err != nil || got != nil {
		t.Errorf("client.Get: %+v, %v", got, err)
	}
}

// registeredError is the one error type of apiWithError.
type registeredError struct {
	Code string `json:"code"`
}

func (registeredError) Error() string { return "registered error" }

type apiWithError struct {
	api.Specification
	api.Register[error, registeredError]

	Get func(context.Context, int) (string, error) `rest:"GET /get/{id=%v}"`
}

func TestClientKeepsAnUndecodableErrorBody(t *testing.T) {
	text := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "disk quota exceeded", http.StatusInsufficientStorage)
	}))
	defer text.Close()
	client := api.Import[apiWithError](rest.API, text.URL, text.Client())
	if _, err := client.Get(context.Background(), 1); err == nil || !strings.Contains(err.Error(), "disk quota") {
		t.Errorf("the server's message was lost: %v", err)
	}
	typed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"code":"nope"}`)
	}))
	defer typed.Close()
	client = api.Import[apiWithError](rest.API, typed.URL, typed.Client())
	_, err := client.Get(context.Background(), 1)
	var got registeredError
	if !errors.As(err, &got) || got.Code != "nope" {
		t.Errorf("a registered error was not decoded: %v", err)
	}
}

func TestClientNoContent(t *testing.T) {
	type API struct {
		api.Specification
		Get   func(context.Context) (*encodedThing, error) `rest:"GET /get"`
		Value func(context.Context) (encodedThing, error)  `rest:"GET /value"`
		Seq   func(context.Context) (iter.Seq[int], error) `rest:"GET /seq"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	if got, err := client.Get(context.Background()); err != nil || got != nil {
		t.Errorf("client.Get: %v, %v", got, err)
	}
	if got, err := client.Value(context.Background()); err != nil || got != (encodedThing{}) {
		t.Errorf("client.Value: %v, %v", got, err)
	}
	seq, err := client.Seq(context.Background())
	if err != nil || seq == nil {
		t.Fatalf("client.Seq: %v, %v", seq != nil, err)
	}
	for range seq {
		t.Errorf("a 204 iterator yielded")
	}
}

func TestClientIteratorResults(t *testing.T) {
	type API struct {
		api.Specification
		Seq   func(context.Context) (iter.Seq[int], error)          `rest:"GET /seq"`
		Empty func(context.Context) (iter.Seq[int], error)          `rest:"GET /empty"`
		Seq2  func(context.Context) (iter.Seq2[string, int], error) `rest:"GET /seq2"`
	}
	h, err := rest.Handler(nil, API{
		Seq: func(context.Context) (iter.Seq[int], error) { return slices.Values([]int{1, 2, 3}), nil },
		Empty: func(context.Context) (iter.Seq[int], error) {
			return func(func(int) bool) {}, nil
		},
		Seq2: func(context.Context) (iter.Seq2[string, int], error) {
			return maps.All(map[string]int{"a": 1, "b": 2}), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	seq, err := client.Seq(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Collect(seq); !slices.Equal(got, []int{1, 2, 3}) {
		t.Errorf("client.Seq: %v", got)
	}
	var stopped []int
	for v := range seq {
		stopped = append(stopped, v)
		break
	}
	if !slices.Equal(stopped, []int{1}) {
		t.Errorf("a stopped iterator went on: %v", stopped)
	}
	empty, err := client.Empty(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Collect(empty); len(got) != 0 {
		t.Errorf("client.Empty: %v", got)
	}
	seq2, err := client.Seq2(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := maps.Collect(seq2); !maps.Equal(got, map[string]int{"a": 1, "b": 2}) {
		t.Errorf("client.Seq2: %v", got)
	}
}

func TestClientRejectsAResponseItCannotDecode(t *testing.T) {
	type API struct {
		api.Specification
		Get func(context.Context) (string, error) `rest:"GET /get"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/form-data; boundary=x")
		io.WriteString(w, "--x--")
	}))
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	defer func() {
		if p := recover(); p != nil {
			t.Errorf("client panicked: %v", p)
		}
	}()
	if _, err := client.Get(context.Background()); err == nil {
		t.Errorf("no error for a content type with no decoder")
	}
}

func TestClientKeepsCredentialsOnItsHost(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skip(err)
	}
	var seen []string
	other := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `"ok"`)
	})}}
	other.Start()
	defer other.Close()
	var hops []string
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops = append(hops, r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/get":
			http.Redirect(w, r, other.URL+"/next", http.StatusFound)
		case "/here":
			http.Redirect(w, r, "/final", http.StatusFound)
		default:
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `"ok"`)
		}
	}))
	defer first.Close()
	type API struct {
		api.Specification
		Get  func(context.Context) (string, error) `rest:"GET /get"`
		Here func(context.Context) (string, error) `rest:"GET /here"`
	}
	client := api.Import[API](rest.API, first.URL, rest.Header("Authorization", "Bearer secret"))
	if got, err := client.Get(context.Background()); err != nil || got != "ok" {
		t.Fatalf("client.Get: %q, %v", got, err)
	}
	if len(seen) != 1 || seen[0] != "" {
		t.Errorf("a redirect to another host saw %q", seen)
	}
	hops = nil
	if got, err := client.Here(context.Background()); err != nil || got != "ok" {
		t.Fatalf("client.Here: %q, %v", got, err)
	}
	if len(hops) != 2 || hops[0] != "Bearer secret" || hops[1] != "Bearer secret" {
		t.Errorf("a redirect on the same host lost the header: %q", hops)
	}
}

func TestHeaderClientLeavesTheRequestAlone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, r.Header.Get("X-Key"))
	}))
	defer srv.Close()
	req, err := http.NewRequest("GET", srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rest.Header("X-Key", "value").Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "value" {
		t.Errorf("the header was not sent: %q", body)
	}
	if got := req.Header.Get("X-Key"); got != "" {
		t.Errorf("the caller's request gained a header: %q", got)
	}
}

func TestClientRemainderEscapesEachSegment(t *testing.T) {
	type API struct {
		api.Specification
		Files func(context.Context, string) (string, error) `rest:"GET /files/{path*=%v}"`
	}
	var escaped string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		io.WriteString(w, `"ok"`)
	}))
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	if _, err := client.Files(context.Background(), "a b/c?d"); err != nil {
		t.Fatal(err)
	}
	if escaped != "/files/a%20b/c%3Fd" {
		t.Errorf("wire path %q", escaped)
	}
}
