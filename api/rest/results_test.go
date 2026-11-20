package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

type namedBytes []byte

// jsonString is text as encoding/json writes a string, which escapes the
// characters HTML would read.
func jsonString(t *testing.T, text string) string {
	t.Helper()
	b, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestBytesAsTextAreTheBytes checks that a []byte sent as text/plain is its
// bytes, the one text a []byte has. Listed a number to a line, as a list of any
// other element is, no client could tell it from a list of numbers, and the
// Go client reading it back into a []byte got the digits.
func TestBytesAsTextAreTheBytes(t *testing.T) {
	type API struct {
		api.Specification
		Bytes   func(context.Context) ([]byte, error)          `rest:"GET /bytes"`
		Named   func(context.Context) (namedBytes, error)      `rest:"GET /named"`
		Raw     func(context.Context) (json.RawMessage, error) `rest:"GET /raw"`
		Binary  func(context.Context) ([]byte, error)          `rest:"GET /binary"`
		Empty   func(context.Context) ([]byte, error)          `rest:"GET /empty"`
		Nil     func(context.Context) ([]byte, error)          `rest:"GET /nil"`
		Pointer func(context.Context) (*[]byte, error)         `rest:"GET /pointer"`
		Lines   func(context.Context) ([][]byte, error)        `rest:"GET /lines"`
		Any     func(context.Context) (any, error)             `rest:"GET /any"`
		Mapped  func(context.Context) (int, []byte, error)     `rest:"GET /mapped n,data"`
		IP      func(context.Context) (net.IP, error)          `rest:"GET /ip"`
		Numbers func(context.Context) ([]int, error)           `rest:"GET /numbers"`
		Shorts  func(context.Context) ([]uint16, error)        `rest:"GET /shorts"`
	}
	hi := []byte("hi")
	h, err := rest.Handler(nil, API{
		Bytes:   func(context.Context) ([]byte, error) { return hi, nil },
		Named:   func(context.Context) (namedBytes, error) { return namedBytes(hi), nil },
		Raw:     func(context.Context) (json.RawMessage, error) { return json.RawMessage(`{"a":1}`), nil },
		Binary:  func(context.Context) ([]byte, error) { return []byte{0xff, 0x00, '\n', 0x80}, nil },
		Empty:   func(context.Context) ([]byte, error) { return []byte{}, nil },
		Nil:     func(context.Context) ([]byte, error) { return nil, nil },
		Pointer: func(context.Context) (*[]byte, error) { return &hi, nil },
		Lines:   func(context.Context) ([][]byte, error) { return [][]byte{[]byte("a"), []byte("b")}, nil },
		Any:     func(context.Context) (any, error) { return hi, nil },
		Mapped:  func(context.Context) (int, []byte, error) { return 2, hi, nil },
		IP:      func(context.Context) (net.IP, error) { return net.IPv4(10, 0, 0, 1), nil },
		Numbers: func(context.Context) ([]int, error) { return []int{104, 105}, nil },
		Shorts:  func(context.Context) ([]uint16, error) { return []uint16{104, 105}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{
		"/bytes":   "hi",
		"/named":   "hi",
		"/raw":     `{"a":1}`,
		"/binary":  "\xff\x00\n\x80",
		"/empty":   "",
		"/nil":     "",
		"/pointer": "hi",
		"/lines":   "a\nb\n",
		"/any":     "hi",
		"/mapped":  "hi",
		// A type with a text form of its own still uses it.
		"/ip": "10.0.0.1",
		// Lists of anything else keep their line to an element.
		"/numbers": "104\n105\n",
		"/shorts":  "104\n105\n",
	} {
		rec := serve(t, h, "GET", target, "", "Accept", "text/plain")
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "text/plain" || rec.Body.String() != want {
			t.Errorf("GET %s: %d ct=%q %q, want %q", target, rec.Code, rec.Header().Get("Content-Type"), rec.Body, want)
		}
	}
	// As JSON a []byte is what encoding/json makes of it, as before.
	if rec := serve(t, h, "GET", "/bytes", "", "Accept", "application/json"); rec.Body.String() != `"aGk="` {
		t.Errorf("GET /bytes as JSON: %d %q", rec.Code, rec.Body)
	}
}

// TestBytesAsTextRoundTrip checks that what the server writes for a []byte as
// text/plain is what a client of this package reads back into one.
func TestBytesAsTextRoundTrip(t *testing.T) {
	type API struct {
		api.Specification
		Echo func(context.Context, []byte) ([]byte, error) `rest:"POST(text/plain) /echo"`
	}
	h, err := rest.Handler(nil, API{
		Echo: func(_ context.Context, b []byte) ([]byte, error) { return b, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	// The Go client asks for JSON, so the header is rewritten on the way in
	// for the answer to be the text/plain under test.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Accept", "text/plain")
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	for _, want := range [][]byte{[]byte("hi"), []byte("two words\nand a line"), {0xff, 0x00, 0x80}} {
		got, err := client.Echo(context.Background(), want)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("client.Echo(%q): %q, %v", want, got, err)
		}
	}
}

// pointerStringer prints itself from a pointer receiver.
type pointerStringer struct{ value string }

func (s *pointerStringer) String() string { return "p:" + s.value }

// pointerProblem is an error by its pointer receiver that is returned as a
// result, as the item of a batch that failed is.
type pointerProblem struct{ value string }

func (p *pointerProblem) Error() string { return "problem:" + p.value }

// hexBytes are bytes with a String method, which is how to show them to people
// and not what a client reads them back from.
type hexBytes []byte

func (h hexBytes) String() string { return fmt.Sprintf("%x", []byte(h)) }

// printedListing is a list by its first field with a String method as well.
type printedListing struct {
	Items []string `json:"items"`
}

func (l *printedListing) String() string { return "a listing" }

// summarised is a list by its first field that has a text form of its own.
type summarised struct {
	Items []string `json:"items"`
}

func (s summarised) MarshalText() ([]byte, error) {
	return fmt.Appendf(nil, "%d items", len(s.Items)), nil
}

// TestTextFormsOnAnyReceiver checks that text/plain shows a value in its own
// text form, whichever receiver the method that gives it is on. Which one that
// is cannot be seen in a function's signature, and a value that printed itself
// until its String method moved to the pointer would otherwise turn into a JSON
// object of its exported fields without a word.
func TestTextFormsOnAnyReceiver(t *testing.T) {
	type API struct {
		api.Specification
		Pointer  func(context.Context) (*pointerStringer, error)   `rest:"GET /pointer"`
		Value    func(context.Context) (*stringered, error)        `rest:"GET /value"`
		Problem  func(context.Context) (*pointerProblem, error)    `rest:"GET /problem"`
		Any      func(context.Context) (any, error)                `rest:"GET /any"`
		List     func(context.Context) ([]*pointerStringer, error) `rest:"GET /list"`
		Nil      func(context.Context) (*pointerStringer, error)   `rest:"GET /nil"`
		NilItem  func(context.Context) ([]*pointerStringer, error) `rest:"GET /nilitem"`
		Hex      func(context.Context) (hexBytes, error)           `rest:"GET /hex"`
		Summary  func(context.Context) (summarised, error)         `rest:"GET /summary"`
		PSummary func(context.Context) (*summarised, error)        `rest:"GET /psummary"`
		Listing  func(context.Context) (*printedListing, error)    `rest:"GET /listing"`
	}
	h, err := rest.Handler(nil, API{
		Pointer: func(context.Context) (*pointerStringer, error) { return &pointerStringer{"a"}, nil },
		Value:   func(context.Context) (*stringered, error) { return &stringered{"a"}, nil },
		Problem: func(context.Context) (*pointerProblem, error) { return &pointerProblem{"a"}, nil },
		Any:     func(context.Context) (any, error) { return &pointerStringer{"a"}, nil },
		List: func(context.Context) ([]*pointerStringer, error) {
			return []*pointerStringer{{"a"}, {"b"}}, nil
		},
		Nil:      func(context.Context) (*pointerStringer, error) { return nil, nil },
		NilItem:  func(context.Context) ([]*pointerStringer, error) { return []*pointerStringer{{"a"}, nil}, nil },
		Hex:      func(context.Context) (hexBytes, error) { return hexBytes("hi"), nil },
		Summary:  func(context.Context) (summarised, error) { return summarised{[]string{"a", "b"}}, nil },
		PSummary: func(context.Context) (*summarised, error) { return &summarised{[]string{"a", "b"}}, nil },
		Listing:  func(context.Context) (*printedListing, error) { return &printedListing{[]string{"a", "b"}}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{
		"/pointer": "p:a",
		"/value":   "s:a",
		"/problem": "problem:a",
		"/any":     "p:a",
		"/list":    "p:a\np:b\n",
		"/nil":     "<nil>",
		"/nilitem": "p:a\n<nil>\n",
		// How a value prints comes before what it is by kind, as it always
		// has, and a struct that wraps a list is that list.
		"/hex":      "6869",
		"/listing":  "a listing",
		"/summary":  "a\nb\n",
		"/psummary": "2 items",
	} {
		rec := serve(t, h, "GET", target, "", "Accept", "text/plain")
		if rec.Code != http.StatusOK || rec.Body.String() != want {
			t.Errorf("GET %s: %d %q, want %q", target, rec.Code, rec.Body, want)
		}
	}
	// A client that asked for nothing in particular is sent the text form of
	// a pointer that has one, as it always was.
	if rec := serve(t, h, "GET", "/pointer", ""); rec.Header().Get("Content-Type") != "text/plain" || rec.Body.String() != "p:a" {
		t.Errorf("GET /pointer with no Accept: ct=%q %q", rec.Header().Get("Content-Type"), rec.Body)
	}
}

// pointerFault is a typed error whose methods are on the pointer receiver.
type pointerFault struct {
	Code string `json:"code"`
}

func (*pointerFault) Error() string { return "fault" }

func (*pointerFault) StatusHTTP() int { return http.StatusConflict }

func (*pointerFault) ContentTypeHTTP() string { return "application/json" }

// taggedError carries another error and an unrelated field, as an application's
// own wrapper does.
type taggedError struct {
	tag string
	err error
}

func (e taggedError) Error() string { return e.tag + ": " + e.err.Error() }

func (e taggedError) Unwrap() error { return e.err }

// htmlFault names a content type the transport has no body for.
type htmlFault struct{}

func (htmlFault) Error() string { return "fault page" }

func (htmlFault) StatusHTTP() int { return http.StatusGone }

func (htmlFault) ContentTypeHTTP() string { return "text/html" }

// TestWrappedErrorsWithoutAJSONBodyKeepTheirMessage checks the neighbours of a
// wrapped JSON error: one that names a type there is no body for, and one that
// names none, are answered with the whole message, wrapping included, and the
// status of the error inside.
func TestWrappedErrorsWithoutAJSONBodyKeepTheirMessage(t *testing.T) {
	type API struct {
		api.Specification
		HTML    func(context.Context) error `rest:"POST /html"`
		Denied  func(context.Context) error `rest:"POST /denied"`
		Plain   func(context.Context) error `rest:"POST /plain"`
		Missing func(context.Context) error `rest:"POST /missing"`
	}
	// An Auth that passes errors on declares their messages safe to
	// show, which is what it takes for them to reach the caller.
	h, err := rest.Handler(identityAuth{}, API{
		HTML:    func(context.Context) error { return fmt.Errorf("wrapped: %w", htmlFault{}) },
		Denied:  func(context.Context) error { return fmt.Errorf("wrapped: %w", deniedError{}) },
		Plain:   func(context.Context) error { return fmt.Errorf("wrapped: %w", errors.New("plain")) },
		Missing: func(context.Context) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]struct {
		status int
		body   string
	}{
		"/html":   {http.StatusGone, "wrapped: fault page\n"},
		"/denied": {http.StatusUnauthorized, "wrapped: denied\n"},
		"/plain":  {http.StatusInternalServerError, "wrapped: plain\n"},
	} {
		rec := serve(t, h, "POST", target, "")
		if rec.Code != want.status || rec.Body.String() != want.body || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
			t.Errorf("POST %s: %d ct=%q %q, want %d %q", target, rec.Code, rec.Header().Get("Content-Type"), rec.Body, want.status, want.body)
		}
	}
	if rec := serve(t, h, "POST", "/missing", ""); rec.Code != http.StatusNoContent {
		t.Errorf("POST /missing: %d %q", rec.Code, rec.Body)
	}
}

// redactingAuth replaces every error with one that says nothing.
type redactingAuth struct{}

func (redactingAuth) Authenticate(ctx context.Context, r *http.Request, fn api.Function) (context.Context, error) {
	return ctx, nil
}

func (redactingAuth) Authorize(context.Context, *http.Request, api.Function, []reflect.Value) error {
	return nil
}

func (redactingAuth) Redact(context.Context, error) error { return errors.New("redacted") }

// TestRedactedErrorsDoNotKeepATypedBody checks that looking inside an error for
// its body does not look past the Auth: once Redact has replaced an error, the
// body the original would have had is not sent.
func TestRedactedErrorsDoNotKeepATypedBody(t *testing.T) {
	type API struct {
		api.Specification
		Bare func(context.Context) error `rest:"POST /bare"`
		Wrap func(context.Context) error `rest:"POST /wrap"`
	}
	h, err := rest.Handler(redactingAuth{}, API{
		Bare: func(context.Context) error { return throttled{Reason: "secret reason"} },
		Wrap: func(context.Context) error {
			return fmt.Errorf("while checking: %w", throttled{Reason: "secret reason"})
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/bare", "/wrap"} {
		rec := serve(t, h, "POST", target, "")
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "redacted\n" {
			t.Errorf("POST %s: %d %q, want 500 redacted", target, rec.Code, rec.Body)
		}
	}
}

// leakyError is what a function fails with: it says too much, in a header as
// much as in its message.
type leakyError struct{}

func (leakyError) Error() string { return "db password=hunter2" }

func (leakyError) WriteHeadersHTTP(h http.Header) { h.Set("X-Debug", "db password=hunter2") }

func (leakyError) RetryAfterHTTP() time.Duration { return 7 * time.Second }

// slowDownError is what an Auth lets the client see in its place.
type slowDownError struct{}

func (slowDownError) Error() string { return "slow down" }

func (slowDownError) StatusHTTP() int { return http.StatusTooManyRequests }

func (slowDownError) RetryAfterHTTP() time.Duration { return 30 * time.Second }

func (slowDownError) WriteHeadersHTTP(h http.Header) { h.Set("X-Public", "yes") }

// replacingAuth redacts every error to the one it holds, and refuses a request
// with the header X-Refuse.
type replacingAuth struct{ with error }

func (replacingAuth) Authenticate(ctx context.Context, r *http.Request, _ api.Function) (context.Context, error) {
	if r.Header.Get("X-Refuse") != "" {
		return ctx, leakyError{}
	}
	return ctx, nil
}

func (replacingAuth) Authorize(context.Context, *http.Request, api.Function, []reflect.Value) error {
	return nil
}

func (a replacingAuth) Redact(context.Context, error) error { return a.with }

// TestRedactedErrorsSendOnlyTheirOwnHeaders checks that a response to an error
// is all from the error Redact returned: the headers and Retry-After of the one
// it replaced say as much as its message would have.
func TestRedactedErrorsSendOnlyTheirOwnHeaders(t *testing.T) {
	type API struct {
		api.Specification
		Fail func(context.Context) (string, error) `rest:"GET /fail"`
		Wrap func(context.Context) (string, error) `rest:"GET /wrap"`
	}
	impl := API{
		Fail: func(context.Context) (string, error) { return "", leakyError{} },
		Wrap: func(context.Context) (string, error) { return "", fmt.Errorf("while reading: %w", leakyError{}) },
	}
	h, err := rest.Handler(replacingAuth{slowDownError{}}, impl)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range [][]string{{"/fail"}, {"/wrap"}, {"/fail", "X-Refuse", "yes"}} {
		rec := serve(t, h, "GET", request[0], "", request[1:]...)
		if rec.Code != http.StatusTooManyRequests || rec.Body.String() != "slow down\n" {
			t.Errorf("GET %v: %d %q, want the redacted 429", request, rec.Code, rec.Body)
		}
		if got := rec.Header().Get("X-Debug"); got != "" {
			t.Errorf("GET %v sent a header of the error that was replaced: X-Debug: %q", request, got)
		}
		if got := rec.Header().Get("X-Public"); got != "yes" {
			t.Errorf("GET %v did not send the header of the error it was redacted to: X-Public: %q", request, got)
		}
		if got := rec.Header().Get("Retry-After"); got != "30" {
			t.Errorf("GET %v: Retry-After %q, want 30", request, got)
		}
	}

	// An error that is redacted to one without headers is sent without any.
	h, err = rest.Handler(replacingAuth{errors.New("redacted")}, impl)
	if err != nil {
		t.Fatal(err)
	}
	rec := serve(t, h, "GET", "/fail", "")
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "redacted\n" {
		t.Errorf("GET /fail: %d %q, want 500 redacted", rec.Code, rec.Body)
	}
	if debug, retry := rec.Header().Get("X-Debug"), rec.Header().Get("Retry-After"); debug != "" || retry != "" {
		t.Errorf("GET /fail kept headers of the error that was replaced: X-Debug %q, Retry-After %q", debug, retry)
	}

	// Without an Auth nothing is replaced, and the error's own headers go out.
	h, err = rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	rec = serve(t, h, "GET", "/wrap", "")
	if debug, retry := rec.Header().Get("X-Debug"), rec.Header().Get("Retry-After"); debug == "" || retry != "7" {
		t.Errorf("GET /wrap without an Auth: X-Debug %q, Retry-After %q", debug, retry)
	}
}

// TestResultRulesAsXML checks that results with RESULT_RULES can be asked for as
// application/xml like any others, which a map of them cannot be written as.
func TestResultRulesAsXML(t *testing.T) {
	type API struct {
		api.Specification
		Pair   func(context.Context) (string, int, error)                    `rest:"GET /pair name,count"`
		Things func(context.Context) (encodedThing, []encodedThing, error)   `rest:"GET /things first,rest"`
		Hidden func(context.Context) (hiddenProfile, []hiddenProfile, error) `rest:"GET /hidden one,many"`
		Nil    func(context.Context) (*encodedThing, any, error)             `rest:"GET /nil thing,other"`
	}
	h, err := rest.Handler(nil, API{
		Pair: func(context.Context) (string, int, error) { return "x", 2, nil },
		Things: func(context.Context) (encodedThing, []encodedThing, error) {
			return encodedThing{"a"}, []encodedThing{{"b"}, {"c"}}, nil
		},
		Hidden: func(context.Context) (hiddenProfile, []hiddenProfile, error) {
			return newHiddenProfile(), []hiddenProfile{newHiddenProfile()}, nil
		},
		Nil: func(context.Context) (*encodedThing, any, error) { return nil, nil, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{
		"/pair":   "<Pair><name>x</name><count>2</count></Pair>",
		"/things": "<Things><first><Name>a</Name></first><rest><Name>b</Name></rest><rest><Name>c</Name></rest></Things>",
		"/nil":    "<Nil></Nil>",
	} {
		rec := serve(t, h, "GET", target, "", "Accept", "application/xml")
		if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/xml" || rec.Body.String() != want {
			t.Errorf("GET %s as XML: %d ct=%q %q, want %q", target, rec.Code, rec.Header().Get("Content-Type"), rec.Body, want)
		}
	}
	rec := serve(t, h, "GET", "/hidden", "", "Accept", "application/xml")
	if rec.Code != 200 || strings.Contains(rec.Body.String(), secret) || !strings.Contains(rec.Body.String(), "<one>") {
		t.Errorf("GET /hidden as XML: %d %q", rec.Code, rec.Body)
	}
	// The other types are written from a map, as they were.
	if rec := serve(t, h, "GET", "/pair", "", "Accept", "application/json"); rec.Body.String() != "{\n\t\"count\": 2,\n\t\"name\": \"x\"\n}" {
		t.Errorf("GET /pair as JSON: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "GET", "/pair", "", "Accept", "text/plain"); rec.Body.String() != `{"count":2,"name":"x"}` {
		t.Errorf("GET /pair as text: %d %q", rec.Code, rec.Body)
	}
}

// TestSeq2ResultsAreAnObjectToAPair checks what an iter.Seq2 is on the wire: a
// list with an object for each pair, so that pairs that share a key, and the
// order of them all, are still there to read.
func TestSeq2ResultsAreAnObjectToAPair(t *testing.T) {
	type API struct {
		api.Specification
		Pairs func(context.Context) (iter.Seq2[int, string], error) `rest:"GET /pairs"`
		Empty func(context.Context) (iter.Seq2[int, string], error) `rest:"GET /empty"`
	}
	h, err := rest.Handler(nil, API{
		Pairs: func(context.Context) (iter.Seq2[int, string], error) {
			return func(yield func(int, string) bool) {
				_ = yield(3, "c") && yield(1, "a") && yield(3, "again")
			}, nil
		},
		Empty: func(context.Context) (iter.Seq2[int, string], error) {
			return func(func(int, string) bool) {}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, accept := range []string{"", "application/json"} {
		rec := serve(t, h, "GET", "/pairs", "", "Accept", accept)
		var pairs []map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &pairs); err != nil {
			t.Fatalf("GET /pairs Accept %q: %d %q: %v", accept, rec.Code, rec.Body, err)
		}
		want := []map[string]string{{"3": "c"}, {"1": "a"}, {"3": "again"}}
		if !reflect.DeepEqual(pairs, want) {
			t.Errorf("GET /pairs Accept %q: %v, want %v", accept, pairs, want)
		}
	}
	// An event stream has always had an event to a pair.
	rec := serve(t, h, "GET", "/pairs", "", "Accept", "text/event-stream")
	if want := "data: {\"3\":\"c\"}\n\ndata: {\"1\":\"a\"}\n\ndata: {\"3\":\"again\"}\n\n"; rec.Body.String() != want {
		t.Errorf("GET /pairs as events: %q, want %q", rec.Body, want)
	}
	if rec := serve(t, h, "GET", "/empty", ""); rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Errorf("GET /empty: %d %q, want 204", rec.Code, rec.Body)
	}
}
