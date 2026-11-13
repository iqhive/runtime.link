package rest_test

import (
	"context"
	"errors"
	"fmt"
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

// TestClientSendsNullForANilPointerBody checks that a nil *T is an argument the
// client can pass for a body: the server takes a body of null for one, so that
// is what has to be sent, whichever way the body travels.
func TestClientSendsNullForANilPointerBody(t *testing.T) {
	type API struct {
		api.Specification
		Update func(context.Context, *encodedThing) (string, error)          `rest:"POST /update"`
		Remove func(context.Context, string, *encodedThing) (string, error)  `rest:"DELETE /things/{id=%v}"`
		Query  func(context.Context, *string, *encodedThing) (string, error) `rest:"POST /query?q=%v"`
		Mapped func(context.Context, *encodedThing, int) (string, error)     `rest:"POST /mapped (thing,n)"`
		Array  func(context.Context, *encodedThing, *int) (string, error)    `rest:"POST /array"`
	}
	// A body of null is a pointer to a zero value to the function, where a
	// null among other arguments is nil.
	name := func(thing *encodedThing) string {
		if thing == nil {
			return "nil"
		}
		return "name:" + thing.Name
	}
	var wire string
	h, err := rest.Handler(nil, API{
		Update: func(_ context.Context, thing *encodedThing) (string, error) { return name(thing), nil },
		Remove: func(_ context.Context, id string, thing *encodedThing) (string, error) {
			return id + " " + name(thing), nil
		},
		Query: func(_ context.Context, q *string, thing *encodedThing) (string, error) {
			return fmt.Sprint(q == nil, " ", name(thing)), nil
		},
		Mapped: func(_ context.Context, thing *encodedThing, n int) (string, error) {
			return fmt.Sprint(name(thing), " ", n), nil
		},
		Array: func(_ context.Context, thing *encodedThing, n *int) (string, error) {
			return fmt.Sprint(name(thing), " ", n == nil), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		wire = string(body)
		r.Body = io.NopCloser(strings.NewReader(wire))
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	ctx := context.Background()
	thing := &encodedThing{"a"}
	for _, tc := range []struct {
		call       string
		got        func() (string, error)
		want, wire string
	}{
		{"Update(nil)", func() (string, error) { return client.Update(ctx, nil) }, "name:", "null"},
		{"Update(thing)", func() (string, error) { return client.Update(ctx, thing) }, "name:a", "{\n\t\"name\": \"a\"\n}"},
		{"Remove(nil)", func() (string, error) { return client.Remove(ctx, "x", nil) }, "x name:", "null"},
		{"Query(nil, nil)", func() (string, error) { return client.Query(ctx, nil, nil) }, "true name:", "null"},
		{"Mapped(nil)", func() (string, error) { return client.Mapped(ctx, nil, 1) }, "nil 1", "{\n\t\"n\": 1\n}"},
		{"Array(nil, nil)", func() (string, error) { return client.Array(ctx, nil, nil) }, "nil true", "[\n\tnull,\n\tnull\n]"},
	} {
		if got, err := tc.got(); err != nil || got != tc.want || wire != tc.wire {
			t.Errorf("client.%s: %q, %v, want %q (wire %q, want %q)", tc.call, got, err, tc.want, wire, tc.wire)
		}
	}
}

// TestClientDeleteOfWhatIsGoneSucceeds pins a deliberate rule of the client: a
// DELETE that is answered 404 has what it asked for, and returns no error,
// where every other method returns one. It holds whoever sent the 404.
func TestClientDeleteOfWhatIsGoneSucceeds(t *testing.T) {
	type API struct {
		api.Specification
		Delete func(context.Context, string) error                 `rest:"DELETE /things/{id=%v}"`
		Get    func(context.Context, string) (encodedThing, error) `rest:"GET /things/{id=%v}"`
		Purge  func(context.Context, string) (string, error)       `rest:"DELETE /purge/{id=%v}"`
		Deny   func(context.Context, string) error                 `rest:"DELETE /deny/{id=%v}"`
	}
	h, err := rest.Handler(nil, API{
		Delete: func(context.Context, string) error { return goneError{} },
		Get:    func(context.Context, string) (encodedThing, error) { return encodedThing{}, goneError{} },
		Purge:  func(context.Context, string) (string, error) { return "", goneError{} },
		Deny:   func(context.Context, string) error { return deniedError{} },
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	ctx := context.Background()
	if err := client.Delete(ctx, "x"); err != nil {
		t.Errorf("client.Delete of what is gone: %v", err)
	}
	if got, err := client.Purge(ctx, "x"); err != nil || got != "" {
		t.Errorf("client.Purge of what is gone: %q, %v", got, err)
	}
	if _, err := client.Get(ctx, "x"); err == nil {
		t.Errorf("client.Get: no error for a 404")
	}
	if err := client.Deny(ctx, "x"); err == nil {
		t.Errorf("client.Deny: no error for a DELETE that was refused")
	}
	elsewhere := httptest.NewServer(http.NotFoundHandler())
	defer elsewhere.Close()
	lost := api.Import[API](rest.API, elsewhere.URL, elsewhere.Client())
	if err := lost.Delete(ctx, "x"); err != nil {
		t.Errorf("client.Delete against a server without the route: %v", err)
	}
}

type goneError struct{}

func (goneError) Error() string { return "no such thing" }

func (goneError) StatusHTTP() int { return http.StatusNotFound }

// TestClientSeq2KeepsEveryPair checks that an iter.Seq2 reaches the caller as it
// was yielded: every pair, a repeated key included, in order.
func TestClientSeq2KeepsEveryPair(t *testing.T) {
	type API struct {
		api.Specification
		Pairs func(context.Context) (iter.Seq2[int, string], error) `rest:"GET /pairs"`
		Empty func(context.Context) (iter.Seq2[int, string], error) `rest:"GET /empty"`
	}
	h, err := rest.Handler(nil, API{
		Pairs: func(context.Context) (iter.Seq2[int, string], error) {
			return func(yield func(int, string) bool) {
				_ = yield(3, "c") && yield(1, "a") && yield(3, "again") && yield(2, "b")
			}, nil
		},
		Empty: func(context.Context) (iter.Seq2[int, string], error) {
			return func(func(int, string) bool) {}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	collect := func(pairs iter.Seq2[int, string], limit int) []string {
		var got []string
		for key, value := range pairs {
			if len(got) == limit {
				break
			}
			got = append(got, fmt.Sprintf("%d=%s", key, value))
		}
		return got
	}
	// Map order would pass this now and then, so it is asked more than once.
	for attempt := range 5 {
		pairs, err := client.Pairs(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got, want := collect(pairs, 10), []string{"3=c", "1=a", "3=again", "2=b"}; !slices.Equal(got, want) {
			t.Fatalf("attempt %d: client.Pairs yielded %v, want %v", attempt, got, want)
		}
		if got, want := collect(pairs, 2), []string{"3=c", "1=a"}; !slices.Equal(got, want) {
			t.Fatalf("attempt %d: a stopped iterator yielded %v, want %v", attempt, got, want)
		}
	}
	empty, err := client.Empty(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := collect(empty, 10); len(got) != 0 {
		t.Errorf("client.Empty yielded %v", got)
	}
}

// TestClientLeavesANilPointerInItsArgument checks that reading the fields of an
// argument for the path and query does not fill in the caller's nil pointers.
func TestClientLeavesANilPointerInItsArgument(t *testing.T) {
	type Paging struct {
		Limit int `json:"limit"`
	}
	type Filter struct {
		Name   string  `json:"name"`
		Paging *Paging `json:"paging"`
	}
	type API struct {
		api.Specification
		List func(context.Context, *Filter) (string, error) `rest:"GET /list?%v"`
		Path func(context.Context, *Filter) (string, error) `rest:"POST /limit/{Paging.Limit}"`
	}
	var wire string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wire = r.RequestURI
		io.WriteString(w, `"ok"`)
	}))
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	filter := &Filter{Name: "x"}
	if _, err := client.List(context.Background(), filter); err != nil || wire != "/list?name=x" {
		t.Errorf("client.List: %v (wire %q)", err, wire)
	}
	if _, err := client.Path(context.Background(), filter); err != nil || wire != "/limit/0" {
		t.Errorf("client.Path: %v (wire %q)", err, wire)
	}
	if filter.Paging != nil {
		t.Errorf("the client filled in its caller's nil pointer: %+v", filter.Paging)
	}
	filter.Paging = &Paging{Limit: 5}
	if _, err := client.List(context.Background(), filter); err != nil || wire != "/list?name=x&paging.limit=5" {
		t.Errorf("client.List with paging: %v (wire %q)", err, wire)
	}
}

// TestClientArgumentRulesAsXML checks that arguments with ARGUMENT_RULES can be
// sent as application/xml, in the elements the server reads them from.
func TestClientArgumentRulesAsXML(t *testing.T) {
	type API struct {
		api.Specification
		Mapped func(context.Context, string, int) (string, error)          `rest:"POST(application/xml) /mapped (a,b)"`
		Things func(context.Context, *encodedThing, []int) (string, error) `rest:"POST(application/xml) /things (thing,n)"`
	}
	h, err := rest.Handler(nil, API{
		Mapped: func(_ context.Context, a string, b int) (string, error) { return fmt.Sprint(a, b), nil },
		Things: func(_ context.Context, thing *encodedThing, n []int) (string, error) {
			return fmt.Sprint(thing, n), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Whatever the root element is called, as it always was.
	if rec := serve(t, h, "POST", "/mapped", "<x><a>s</a><b>2</b></x>", "Content-Type", "application/xml"); rec.Code != 200 || rec.Body.String() != "s2" {
		t.Fatalf("POST /mapped: %d %q", rec.Code, rec.Body)
	}
	var wire string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		wire = string(body)
		r.Body = io.NopCloser(strings.NewReader(wire))
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	if got, err := client.Mapped(context.Background(), "s", 2); err != nil || got != "s2" || wire != "<Mapped><a>s</a><b>2</b></Mapped>" {
		t.Errorf("client.Mapped: %q, %v (wire %q)", got, err, wire)
	}
	if got, err := client.Things(context.Background(), &encodedThing{"x"}, []int{1, 2}); err != nil || got != "&{x} [1 2]" {
		t.Errorf("client.Things: %q, %v (wire %q)", got, err, wire)
	}
	if got, err := client.Things(context.Background(), nil, nil); err != nil || got != "<nil> []" {
		t.Errorf("client.Things(nil, nil): %q, %v (wire %q)", got, err, wire)
	}
}

// TestClientDecodesARegisteredErrorAheadOfTrailingData pins that a body which
// starts with the registered error is that error, whatever follows it, as it
// always was. A body that starts with null is still no error, but a message.
func TestClientDecodesARegisteredErrorAheadOfTrailingData(t *testing.T) {
	for body, want := range map[string]string{
		`{"code":"nope"} and more`:        "nope",
		`{"code":"nope"}{"code":"again"}`: "nope",
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, body)
		}))
		client := api.Import[apiWithError](rest.API, srv.URL, srv.Client())
		_, err := client.Get(context.Background(), 1)
		var got registeredError
		if !errors.As(err, &got) || got.Code != want {
			t.Errorf("body %q: %v", body, err)
		}
		srv.Close()
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "null pointer dereference")
	}))
	defer srv.Close()
	client := api.Import[apiWithError](rest.API, srv.URL, srv.Client())
	if _, err := client.Get(context.Background(), 1); err == nil || !strings.Contains(err.Error(), "null pointer dereference") {
		t.Errorf("the server's message was lost: %v", err)
	}
}
