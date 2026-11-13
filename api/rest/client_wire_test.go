package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

// wireLog is the requests a server was sent, as they were sent.
type wireLog struct {
	mu       sync.Mutex
	requests []wireRequest
}

type wireRequest struct {
	method, path, query, body string
}

func (log *wireLog) all() []wireRequest {
	log.mu.Lock()
	defer log.mu.Unlock()
	return slices.Clone(log.requests)
}

func (log *wireLog) last(t *testing.T) wireRequest {
	t.Helper()
	requests := log.all()
	if len(requests) == 0 {
		t.Fatal("the server was sent nothing")
	}
	return requests[len(requests)-1]
}

// linked serves impl and returns a client of the same API that is linked to it,
// with what the client sends.
func linked[API any](t *testing.T, impl API) (API, *wireLog) {
	t.Helper()
	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	log := new(wireLog)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		log.mu.Lock()
		log.requests = append(log.requests, wireRequest{r.Method, r.URL.EscapedPath(), r.URL.RawQuery, string(body)})
		log.mu.Unlock()
		r.Body = io.NopCloser(bytes.NewReader(body))
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return api.Import[API](rest.API, srv.URL, srv.Client()), log
}

type pathKey struct {
	ID string `json:"id"`
}

type pathClient struct {
	api.Specification

	DeleteAll func(context.Context) error                       `rest:"DELETE /things"`
	Delete    func(context.Context, string) error               `rest:"DELETE /things/{id=%v}"`
	List      func(context.Context) (string, error)             `rest:"GET /things"`
	Get       func(context.Context, string) (string, error)     `rest:"GET /things/{id=%v}"`
	GetPtr    func(context.Context, *string) (string, error)    `rest:"GET /ptr/{id=%v}"`
	Number    func(context.Context, int) (string, error)        `rest:"GET /numbers/{n=%v}"`
	Files     func(context.Context, string) (string, error)     `rest:"GET /files/{path*=%v}"`
	Tagged    func(context.Context, []string) (string, error)   `rest:"GET /tagged/{tags=%v}"`
	Keyed     func(context.Context, pathKey) (string, error)    `rest:"GET /keyed/{ID}"`
	Watch     func(context.Context, string) (<-chan int, error) `rest:"GET /watch/{id=%v}"`
}

func newPathClient() pathClient {
	return pathClient{
		DeleteAll: func(context.Context) error { return nil },
		Delete:    func(context.Context, string) error { return nil },
		List:      func(context.Context) (string, error) { return "all of them", nil },
		Get:       func(_ context.Context, id string) (string, error) { return "one:" + id, nil },
		GetPtr: func(_ context.Context, id *string) (string, error) {
			if id == nil {
				return "nil", nil
			}
			return "ptr:" + *id, nil
		},
		Number: func(_ context.Context, n int) (string, error) { return fmt.Sprint("n:", n), nil },
		Files:  func(_ context.Context, path string) (string, error) { return "file:" + path, nil },
		Tagged: func(_ context.Context, tags []string) (string, error) { return fmt.Sprintf("tags:%q", tags), nil },
		Keyed:  func(_ context.Context, key pathKey) (string, error) { return "key:" + key.ID, nil },
		Watch: func(context.Context, string) (<-chan int, error) {
			values := make(chan int)
			close(values)
			return values, nil
		},
	}
}

// TestClientRefusesAnEmptyPathValue checks that a call whose path value has
// nothing in it is not sent: DELETE /things/ is DELETE /things to a server, so
// deleting the thing called "" would delete all of them, and a nil pointer would
// be sent as the placeholder it was to replace.
func TestClientRefusesAnEmptyPathValue(t *testing.T) {
	client, log := linked(t, newPathClient())
	ctx := context.Background()
	empty := ""
	calls := map[string]func() error{
		"Delete an empty string":     func() error { return client.Delete(ctx, "") },
		"Get an empty string":        func() error { _, err := client.Get(ctx, ""); return err },
		"GetPtr nil":                 func() error { _, err := client.GetPtr(ctx, nil); return err },
		"GetPtr an empty string":     func() error { _, err := client.GetPtr(ctx, &empty); return err },
		"Files an empty remainder":   func() error { _, err := client.Files(ctx, ""); return err },
		"Tagged nil":                 func() error { _, err := client.Tagged(ctx, nil); return err },
		"Tagged no elements":         func() error { _, err := client.Tagged(ctx, []string{}); return err },
		"Tagged one empty element":   func() error { _, err := client.Tagged(ctx, []string{""}); return err },
		"Keyed an empty field":       func() error { _, err := client.Keyed(ctx, pathKey{}); return err },
		"Watch an empty string":      func() error { _, err := client.Watch(ctx, ""); return err },
		"Delete after a working one": func() error { client.Delete(ctx, "x"); return client.Delete(ctx, "") },
	}
	for name, call := range calls {
		before := len(log.all())
		if name == "Delete after a working one" {
			before++
		}
		err := call()
		if err == nil || !strings.Contains(err.Error(), "path parameter") {
			t.Errorf("%s: err = %v, want one about the path parameter", name, err)
		}
		if sent := log.all(); len(sent) != before {
			t.Errorf("%s: the server was sent %+v", name, sent[len(sent)-1])
		}
	}
}

// TestClientSendsPathValuesThatAreNotEmpty checks the values next to the empty
// one, which are all values a path carries.
func TestClientSendsPathValuesThatAreNotEmpty(t *testing.T) {
	client, log := linked(t, newPathClient())
	ctx := context.Background()
	space, zero := " ", "0"
	for _, c := range []struct {
		call       func() (string, error)
		path, want string
	}{
		{func() (string, error) { return client.Get(ctx, "5") }, "/things/5", "one:5"},
		{func() (string, error) { return client.Get(ctx, " ") }, "/things/%20", "one: "},
		{func() (string, error) { return client.Get(ctx, "{id}") }, "/things/%7Bid%7D", "one:{id}"},
		{func() (string, error) { return client.GetPtr(ctx, &space) }, "/ptr/%20", "ptr: "},
		{func() (string, error) { return client.GetPtr(ctx, &zero) }, "/ptr/0", "ptr:0"},
		{func() (string, error) { return client.Number(ctx, 0) }, "/numbers/0", "n:0"},
		{func() (string, error) { return client.Files(ctx, "a/b") }, "/files/a/b", "file:a/b"},
		{func() (string, error) { return client.Files(ctx, "a/") }, "/files/a/", "file:a/"},
		{func() (string, error) { return client.Tagged(ctx, []string{"", "a"}) }, "/tagged/,a", `tags:["" "a"]`},
		{func() (string, error) { return client.Keyed(ctx, pathKey{ID: "k"}) }, "/keyed/k", "key:k"},
		{func() (string, error) { return client.List(ctx) }, "/things", "all of them"},
	} {
		got, err := c.call()
		if err != nil || got != c.want {
			t.Errorf("%s: %q, %v; want %q", c.path, got, err, c.want)
		}
		if sent := log.last(t); sent.path != c.path {
			t.Errorf("sent %q, want %q", sent.path, c.path)
		}
	}
}

type rawQuery struct {
	api.Specification

	Find func(context.Context, json.RawMessage) (string, error) `rest:"GET /find?q=%v"`
}

// escaped is the escape JSON has for a UTF-16 code unit, by its four hex digits.
func escaped(unit string) string { return `\` + "u" + unit }

// TestQueryValuesAreReadAsJSONStrings checks that a value whose text is the JSON
// it makes of itself loses only the quotes of a string: JSON has escapes that Go
// does not, and unquoted the Go way a string that used one was sent as nothing.
func TestQueryValuesAreReadAsJSONStrings(t *testing.T) {
	client, log := linked(t, rawQuery{
		Find: func(_ context.Context, q json.RawMessage) (string, error) { return string(q), nil },
	})
	for _, c := range []struct{ raw, sent, want string }{
		{`"plain"`, "q=plain", `"plain"`},
		{`"a\/b"`, "q=a%2Fb", `"a/b"`},
		{`"` + escaped("d83d") + escaped("de00") + `"`, "q=%F0%9F%98%80", `"😀"`},
		{`"tab\there"`, "q=tab%09here", `"tab\there"`},
		{`"` + escaped("00e9") + `"`, "q=%C3%A9", `"é"`},
		{`""`, "q=", ""},
		{`{"a":1}`, "q=%7B%22a%22%3A1%7D", `{"a":1}`},
		{`12`, "q=12", `12`},
		// Not a JSON string at all, and sent as what it is.
		{`"open`, "q=%22open", `"\"open"`},
	} {
		got, err := client.Find(context.Background(), json.RawMessage(c.raw))
		if err != nil || got != c.want {
			t.Errorf("Find(%s) = %q, %v; want %q", c.raw, got, err, c.want)
		}
		if sent := log.last(t); sent.query != c.sent {
			t.Errorf("Find(%s) sent %q, want %q", c.raw, sent.query, c.sent)
		}
	}
}

type bodyFilter struct {
	Name string `json:"name"`
}

type bodiedMethods struct {
	api.Specification

	Search   func(context.Context, bodyFilter) (string, error)  `rest:"GET /search"`
	Probe    func(context.Context, bodyFilter) error            `rest:"HEAD /search"`
	Allowed  func(context.Context, bodyFilter) (string, error)  `rest:"OPTIONS /search"`
	Remove   func(context.Context, bodyFilter) (string, error)  `rest:"DELETE /search"`
	Optional func(context.Context, *bodyFilter) (string, error) `rest:"GET /optional"`

	Plain   func(context.Context) (string, error)             `rest:"GET /plain"`
	Carried func(context.Context, bodyFilter) (string, error) `rest:"GET /carried/{Name}"`
	Queried func(context.Context, bodyFilter) (string, error) `rest:"GET /queried?%v"`
}

// TestMethodsWithoutABodySendAnArgumentThatNeedsOne checks that an argument the
// path and query have no place for is sent as the body whatever the method: the
// server reads one for it, so without it the function could never be called.
func TestMethodsWithoutABodySendAnArgumentThatNeedsOne(t *testing.T) {
	var probed string
	named := func(prefix string) func(context.Context, bodyFilter) (string, error) {
		return func(_ context.Context, f bodyFilter) (string, error) { return prefix + f.Name, nil }
	}
	client, log := linked(t, bodiedMethods{
		Search:  named("found "),
		Probe:   func(_ context.Context, f bodyFilter) error { probed = f.Name; return nil },
		Allowed: named("allowed "),
		Remove:  named("removed "),
		Optional: func(_ context.Context, f *bodyFilter) (string, error) {
			return fmt.Sprint("nil:", f == nil), nil
		},
		Plain:   func(context.Context) (string, error) { return "plain", nil },
		Carried: named("carried "),
		Queried: named("queried "),
	})
	ctx := context.Background()
	const body = "{\n\t\"name\": \"x\"\n}"
	for _, c := range []struct {
		call         func() (string, error)
		method, want string
		body         string
	}{
		{func() (string, error) { return client.Search(ctx, bodyFilter{Name: "x"}) }, "GET", "found x", body},
		{func() (string, error) { return client.Allowed(ctx, bodyFilter{Name: "x"}) }, "OPTIONS", "allowed x", body},
		{func() (string, error) { return client.Remove(ctx, bodyFilter{Name: "x"}) }, "DELETE", "removed x", body},
		{func() (string, error) { return client.Optional(ctx, nil) }, "GET", "nil:false", "null"},
		// What has no argument for the body still has no body.
		{func() (string, error) { return client.Plain(ctx) }, "GET", "plain", ""},
		{func() (string, error) { return client.Carried(ctx, bodyFilter{Name: "x"}) }, "GET", "carried x", ""},
		{func() (string, error) { return client.Queried(ctx, bodyFilter{Name: "x"}) }, "GET", "queried x", ""},
	} {
		got, err := c.call()
		if err != nil || got != c.want {
			t.Errorf("%s: %q, %v; want %q", c.want, got, err, c.want)
		}
		if sent := log.last(t); sent.method != c.method || sent.body != c.body {
			t.Errorf("%s: sent %s with the body %q, want %s with %q", c.want, sent.method, sent.body, c.method, c.body)
		}
	}
	if err := client.Probe(ctx, bodyFilter{Name: "x"}); err != nil || probed != "x" {
		t.Errorf("Probe: %v, the function got %q", err, probed)
	}
}

type xmlBodies struct {
	api.Specification

	Update func(context.Context, *bodyFilter) (string, error) `rest:"POST(application/xml) /update"`
	Value  func(context.Context, bodyFilter) (string, error)  `rest:"POST(application/xml) /value"`
	Count  func(context.Context, *int) (string, error)        `rest:"POST(text/plain) /count"`
	Text   func(context.Context, *string) (string, error)     `rest:"POST(text/plain) /text"`
	Lines  func(context.Context, *[]string) (string, error)   `rest:"POST(text/plain) /lines"`
}

func newXMLBodies() xmlBodies {
	return xmlBodies{
		Update: func(_ context.Context, f *bodyFilter) (string, error) {
			if f == nil {
				return "nil", nil
			}
			return "set " + f.Name, nil
		},
		Value: func(_ context.Context, f bodyFilter) (string, error) { return "value " + f.Name, nil },
		Count: func(_ context.Context, n *int) (string, error) {
			if n == nil {
				return "nil", nil
			}
			return fmt.Sprint("count ", *n), nil
		},
		Text: func(_ context.Context, s *string) (string, error) {
			if s == nil {
				return "nil", nil
			}
			return fmt.Sprintf("text %q", *s), nil
		},
		Lines: func(_ context.Context, lines *[]string) (string, error) {
			if lines == nil {
				return "nil", nil
			}
			return fmt.Sprintf("lines %q", *lines), nil
		},
	}
}

// TestClientPassesANilPointerBodyOutsideJSON checks that a nil *T is an argument
// in the types that have no null to send for it. It is sent as no body, which
// the server took for a body it could not read, and is nil again to the
// function, unless nothing is a value of the type.
func TestClientPassesANilPointerBodyOutsideJSON(t *testing.T) {
	client, log := linked(t, newXMLBodies())
	ctx := context.Background()
	five, empty, word := 5, "", "word"
	for _, c := range []struct {
		call func() (string, error)
		want string
		body string
	}{
		{func() (string, error) { return client.Update(ctx, nil) }, "nil", ""},
		{func() (string, error) { return client.Update(ctx, &bodyFilter{Name: "x"}) }, "set x", "<bodyFilter><Name>x</Name></bodyFilter>"},
		{func() (string, error) { return client.Update(ctx, &bodyFilter{}) }, "set ", "<bodyFilter><Name></Name></bodyFilter>"},
		{func() (string, error) { return client.Count(ctx, nil) }, "nil", ""},
		{func() (string, error) { return client.Count(ctx, &five) }, "count 5", "5"},
		// Nothing is a string, and a list with nothing in it, so those are
		// what an empty body has always been read as.
		{func() (string, error) { return client.Text(ctx, nil) }, `text ""`, ""},
		{func() (string, error) { return client.Text(ctx, &empty) }, `text ""`, ""},
		{func() (string, error) { return client.Text(ctx, &word) }, `text "word"`, "word"},
		{func() (string, error) { return client.Lines(ctx, nil) }, `lines []`, ""},
	} {
		got, err := c.call()
		if err != nil || got != c.want {
			t.Errorf("%q, %v; want %q", got, err, c.want)
		}
		if sent := log.last(t); sent.body != c.body {
			t.Errorf("%s: the body sent was %q, want %q", c.want, sent.body, c.body)
		}
	}
}

// TestAnEmptyBodyIsOnlyNilToAPointer checks what an empty body is not: an
// argument that is no pointer still needs a body, and a body of space is one
// that cannot be read.
func TestAnEmptyBodyIsOnlyNilToAPointer(t *testing.T) {
	h, err := rest.Handler(nil, newXMLBodies())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		target, ctype, body string
		code                int
		want                string
	}{
		{"/update", "application/xml", "", 200, "nil"},
		{"/update", "application/xml", " ", 400, ""},
		{"/update", "application/xml", "<bodyFilter><Name>x</Name></bodyFilter>", 200, "set x"},
		{"/update", "application/xml", "<bodyFilter>", 400, ""},
		{"/value", "application/xml", "", 400, ""},
		{"/count", "text/plain", "", 200, "nil"},
		{"/count", "text/plain", "x", 400, ""},
		// As JSON a nil pointer is null, and no body at all is still not JSON.
		{"/update", "application/json", "", 400, ""},
		{"/update", "application/json", "null", 200, "set "},
	} {
		rec := serve(t, h, "POST", c.target, c.body, "Content-Type", c.ctype)
		if rec.Code != c.code || c.code == 200 && rec.Body.String() != c.want {
			t.Errorf("POST %s as %s with %q: %d %q, want %d %q", c.target, c.ctype, c.body, rec.Code, rec.Body, c.code, c.want)
		}
	}
}

// jsonSet is a slice that is one value to its JSON form, which is how a query
// carries it.
type jsonSet []string

func (s jsonSet) MarshalJSON() ([]byte, error) { return json.Marshal(strings.Join(s, "+")) }

func (s *jsonSet) UnmarshalJSON(b []byte) error {
	var text string
	if err := json.Unmarshal(b, &text); err != nil {
		return err
	}
	*s = strings.Split(text, "+")
	return nil
}

// jsonPair is a slice that reads itself from a JSON array and writes itself as
// one, by default.
type jsonPair []int

func (p *jsonPair) UnmarshalJSON(b []byte) error {
	var values []int
	if err := json.Unmarshal(b, &values); err != nil {
		return err
	}
	if len(values) != 2 {
		return fmt.Errorf("a pair has two values, not %d", len(values))
	}
	*p = values
	return nil
}

type textSets struct {
	api.Specification

	Query  func(context.Context, jsonSet) (string, error)  `rest:"GET /set?s=%v"`
	Body   func(context.Context, jsonSet) (string, error)  `rest:"POST(text/plain) /set"`
	Pair   func(context.Context, jsonPair) (string, error) `rest:"POST(text/plain) /pair"`
	List   func(context.Context, []string) (string, error) `rest:"POST(text/plain) /list"`
	Result func(context.Context) (jsonSet, error)          `rest:"GET(text/plain) /result"`
}

// TestTextBodyOfASliceThatReadsItself checks that a slice which is one value to
// its UnmarshalJSON is written as one in a text/plain body, as it is in a
// query: written a line to an element, the server read the lines as the value.
func TestTextBodyOfASliceThatReadsItself(t *testing.T) {
	join := func(_ context.Context, s jsonSet) (string, error) { return strings.Join(s, ","), nil }
	impl := textSets{
		Query:  join,
		Body:   join,
		Pair:   func(_ context.Context, p jsonPair) (string, error) { return fmt.Sprint(p), nil },
		List:   func(_ context.Context, l []string) (string, error) { return strings.Join(l, ","), nil },
		Result: func(context.Context) (jsonSet, error) { return jsonSet{"a", "b"}, nil },
	}
	client, log := linked(t, impl)
	ctx := context.Background()
	for _, c := range []struct {
		call       func() (string, error)
		body, want string
	}{
		{func() (string, error) { return client.Query(ctx, jsonSet{"a", "b"}) }, "", "a,b"},
		{func() (string, error) { return client.Body(ctx, jsonSet{"a", "b"}) }, "a+b", "a,b"},
		{func() (string, error) { return client.Body(ctx, jsonSet{"one"}) }, "one", "one"},
		{func() (string, error) { return client.Pair(ctx, jsonPair{1, 2}) }, "[1,2]", "[1 2]"},
		// A slice that does not read itself is a list, a line to an element.
		{func() (string, error) { return client.List(ctx, []string{"a", "b"}) }, "a\nb\n", "a,b"},
	} {
		got, err := c.call()
		if err != nil || got != c.want {
			t.Errorf("%q, %v; want %q", got, err, c.want)
		}
		if sent := log.last(t); sent.body != c.body {
			t.Errorf("%s: the body sent was %q, want %q", c.want, sent.body, c.body)
		}
	}
	// A result is written the same way, and read back by the client.
	if got, err := client.Result(ctx); err != nil || !slices.Equal(got, jsonSet{"a", "b"}) {
		t.Errorf("Result = %q, %v", got, err)
	}
	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "GET", "/result", "", "Accept", "text/plain"); rec.Code != 200 || rec.Body.String() != "a+b" {
		t.Errorf("GET /result as text: %d %q, want \"a+b\"", rec.Code, rec.Body)
	}
}

type keyedSequences struct {
	api.Specification

	Timeline func(context.Context) (iter.Seq2[time.Time, int], error)  `rest:"GET /timeline"`
	Counts   func(context.Context) (iter.Seq2[int, string], error)     `rest:"GET /counts"`
	Names    func(context.Context) (iter.Seq2[string, int], error)     `rest:"GET /names"`
	Optional func(context.Context) (iter.Seq2[*time.Time, int], error) `rest:"GET /optional"`
	Form     func(context.Context) (map[time.Time]string, error)       `rest:"GET /form"`
}

// TestSequenceKeysAreWrittenAsTheirText checks that the key of an iter.Seq2 is
// written as the text its type makes of itself, which is what a client reads it
// back from: what fmt prints for a time.Time is nothing that parses as one.
func TestSequenceKeysAreWrittenAsTheirText(t *testing.T) {
	when := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
	later := when.Add(time.Hour)
	impl := keyedSequences{
		Timeline: func(context.Context) (iter.Seq2[time.Time, int], error) {
			return func(yield func(time.Time, int) bool) { _ = yield(when, 1) && yield(later, 2) }, nil
		},
		Counts: func(context.Context) (iter.Seq2[int, string], error) {
			return func(yield func(int, string) bool) { _ = yield(-7, "a") && yield(-7, "b") }, nil
		},
		Names: func(context.Context) (iter.Seq2[string, int], error) {
			return func(yield func(string, int) bool) { yield("a b", 1) }, nil
		},
		Optional: func(context.Context) (iter.Seq2[*time.Time, int], error) {
			return func(yield func(*time.Time, int) bool) { _ = yield(nil, 1) && yield(&when, 2) }, nil
		},
		Form: func(context.Context) (map[time.Time]string, error) {
			return map[time.Time]string{when: "then"}, nil
		},
	}
	client, _ := linked(t, impl)
	ctx := context.Background()

	timeline, err := client.Timeline(ctx)
	if err != nil {
		t.Fatalf("Timeline: %v", err)
	}
	var times []time.Time
	for key := range timeline {
		times = append(times, key)
	}
	if len(times) != 2 || !times[0].Equal(when) || !times[1].Equal(later) {
		t.Errorf("Timeline keys %v, want [%v %v]", times, when, later)
	}
	counts, err := client.Counts(ctx)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	var pairs []string
	for key, value := range counts {
		pairs = append(pairs, fmt.Sprintf("%d %s", key, value))
	}
	if !slices.Equal(pairs, []string{"-7 a", "-7 b"}) {
		t.Errorf("Counts pairs %q", pairs)
	}

	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{
		"/timeline": `[{"2024-05-06T07:08:09Z":1},{"2024-05-06T08:08:09Z":2}]`,
		"/names":    `[{"a b":1}]`,
		// A nil key has no text, and is still not a panic.
		"/optional": `[{"<nil>":1},{"2024-05-06T07:08:09Z":2}]`,
	} {
		rec := serve(t, h, "GET", target, "", "Accept", "application/json")
		var got, wanted []map[string]int
		if err := json.Unmarshal([]byte(want), &wanted); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != 200 || !reflect.DeepEqual(got, wanted) {
			t.Errorf("GET %s: %d %s (%v), want %s", target, rec.Code, rec.Body, err, want)
		}
	}
	// The fields of a form are named by the keys of a map in the same way.
	rec := serve(t, h, "GET", "/form", "", "Accept", "multipart/form-data")
	_, params, err := mime.ParseMediaType(rec.Header().Get("Content-Type"))
	if err != nil {
		t.Fatalf("GET /form: %d, %v", rec.Code, err)
	}
	form, err := multipart.NewReader(rec.Body, params["boundary"]).ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := form.Value["2024-05-06T07:08:09Z"]; !slices.Equal(got, []string{"then"}) {
		t.Errorf("the form has the fields %v", form.Value)
	}
}
