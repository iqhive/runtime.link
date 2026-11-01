package rest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

func TestNumbersParseStrictly(t *testing.T) {
	type API struct {
		api.Specification
		N func(context.Context, int) (int, error)         `rest:"GET /n?n=%v"`
		P func(context.Context, int) (int, error)         `rest:"GET /p/{id=%v}"`
		U func(context.Context, uint8) (uint8, error)     `rest:"GET /u?n=%v"`
		F func(context.Context, float64) (float64, error) `rest:"GET /f?n=%v"`
		B func(context.Context, bool) (bool, error)       `rest:"GET /b?n=%v"`
	}
	h, err := rest.Handler(nil, API{
		N: func(_ context.Context, n int) (int, error) { return n, nil },
		P: func(_ context.Context, n int) (int, error) { return n, nil },
		U: func(_ context.Context, n uint8) (uint8, error) { return n, nil },
		F: func(_ context.Context, n float64) (float64, error) { return n, nil },
		B: func(_ context.Context, n bool) (bool, error) { return n, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		target string
		status int
		want   string
	}{
		{"/n?n=10", 200, "10"},
		{"/n?n=-3", 200, "-3"},
		{"/n?n=010", 200, "10"},
		{"/n?n=09", 200, "9"},
		{"/n?n=0x10", 400, ""},
		{"/n?n=0b11", 400, ""},
		{"/n?n=1_000", 400, ""},
		{"/n?n=12abc", 400, ""},
		{"/n?n=1.9", 400, ""},
		{"/n?n=9999999999999999999999", 400, ""},
		{"/p/12abc", 400, ""},
		{"/p/007", 200, "7"},
		{"/p/0089", 200, "89"},
		{"/u?n=255", 200, "255"},
		{"/u?n=256", 400, ""},
		{"/u?n=-1", 400, ""},
		{"/f?n=1.5", 200, "1.5"},
		{"/f?n=1.5x", 400, ""},
		{"/b?n=true", 200, "true"},
		{"/b?n=yes", 400, ""},
		// An empty value is an omitted one.
		{"/n?n=", 200, "0"},
	} {
		rec := serve(t, h, "GET", tc.target, "")
		if rec.Code != tc.status {
			t.Errorf("GET %s: %d %q, want %d", tc.target, rec.Code, rec.Body, tc.status)
			continue
		}
		if tc.status == 200 && rec.Body.String() != tc.want {
			t.Errorf("GET %s: %q, want %q", tc.target, rec.Body, tc.want)
		}
	}
}

func TestDurationParameters(t *testing.T) {
	type API struct {
		api.Specification
		Q func(context.Context, time.Duration) (string, error) `rest:"GET /q?d=%v"`
		P func(context.Context, time.Duration) (string, error) `rest:"GET /p/{d=%v}"`
	}
	h, err := rest.Handler(nil, API{
		Q: func(_ context.Context, d time.Duration) (string, error) { return d.String(), nil },
		P: func(_ context.Context, d time.Duration) (string, error) { return d.String(), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	if got, err := client.Q(context.Background(), 90*time.Minute); got != "1h30m0s" {
		t.Errorf("query duration: %q, %v", got, err)
	}
	if got, err := client.P(context.Background(), 90*time.Minute); got != "1h30m0s" {
		t.Errorf("path duration: %q, %v", got, err)
	}
	if rec := serve(t, h, "GET", "/q?d=soon", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("GET /q?d=soon: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "GET", "/q?d=1500", ""); rec.Body.String() != "1.5µs" {
		t.Errorf("a bare integer stays nanoseconds, as JSON has it: %d %q", rec.Code, rec.Body)
	}
}

func TestTimeInAPath(t *testing.T) {
	type API struct {
		api.Specification
		P func(context.Context, time.Time) (string, error) `rest:"GET /p/{t=%v}"`
		Q func(context.Context, time.Time) (string, error) `rest:"GET /q?t=%v"`
	}
	format := func(_ context.Context, ti time.Time) (string, error) { return ti.UTC().Format(time.RFC3339), nil }
	h, err := rest.Handler(nil, API{P: format, Q: format})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if got, err := client.P(context.Background(), when); err != nil || got != "2026-01-02T03:04:05Z" {
		t.Errorf("path time: %q, %v", got, err)
	}
	if got, err := client.Q(context.Background(), when); err != nil || got != "2026-01-02T03:04:05Z" {
		t.Errorf("query time: %q, %v", got, err)
	}
}

// validatedEmail is a string whose text form is validated.
type validatedEmail string

func (e *validatedEmail) UnmarshalText(b []byte) error {
	if !strings.Contains(string(b), "@") {
		return fmt.Errorf("not an email")
	}
	*e = validatedEmail(strings.ToLower(string(b)))
	return nil
}

func TestStringKindsUseUnmarshalText(t *testing.T) {
	type API struct {
		api.Specification
		Q func(context.Context, validatedEmail) (string, error) `rest:"GET /q?e=%v"`
	}
	h, err := rest.Handler(nil, API{
		Q: func(_ context.Context, e validatedEmail) (string, error) { return string(e), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "GET", "/q?e=nonsense", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("validation skipped: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "GET", "/q?e=A@B", ""); rec.Body.String() != "a@b" {
		t.Errorf("GET /q?e=A@B: %d %q", rec.Code, rec.Body)
	}
}

// udpPort implements fmt.Scanner, the only way some types read text.
type udpPort struct{ n int }

func (p *udpPort) Scan(state fmt.ScanState, verb rune) error {
	_, err := fmt.Fscan(state, &p.n)
	return err
}

func TestTextForms(t *testing.T) {
	type API struct {
		api.Specification
		IP      func(context.Context, net.IP) (string, error)          `rest:"GET /ip?v=%v"`
		Scanner func(context.Context, udpPort) (int, error)            `rest:"GET /scan?v=%v"`
		Raw     func(context.Context, json.RawMessage) (string, error) `rest:"GET /raw?v=%v"`
	}
	h, err := rest.Handler(nil, API{
		IP:      func(_ context.Context, ip net.IP) (string, error) { return ip.String(), nil },
		Scanner: func(_ context.Context, p udpPort) (int, error) { return p.n, nil },
		Raw:     func(_ context.Context, r json.RawMessage) (string, error) { return string(r), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{
		"/ip?v=10.0.0.1": "10.0.0.1",
		"/scan?v=8080":   "8080",
		"/raw?v=7":       "7",
	} {
		if rec := serve(t, h, "GET", target, ""); rec.Body.String() != want {
			t.Errorf("GET %s: %d %q, want %q", target, rec.Code, rec.Body, want)
		}
	}
}

type pointerThing struct {
	Name string `json:"name"`
}

func TestPointerArguments(t *testing.T) {
	type Filter struct {
		Limit int `json:"limit"`
	}
	type API struct {
		api.Specification
		Query  func(context.Context, *int) (string, error)               `rest:"GET /query?limit=%v"`
		Path   func(context.Context, *int) (string, error)               `rest:"GET /path/{id=%v}"`
		PStr   func(context.Context, *string) (string, error)            `rest:"GET /pstr?s=%v"`
		PTime  func(context.Context, *time.Time) (string, error)         `rest:"GET /ptime?t=%v"`
		Expand func(context.Context, *Filter) (string, error)            `rest:"GET /expand?%v"`
		Body   func(context.Context, *pointerThing) (string, error)      `rest:"POST /body"`
		Map    func(context.Context, *pointerThing, int) (string, error) `rest:"POST /map (thing,n)"`
		Nested func(context.Context, *pointerThing) (string, error)      `rest:"POST /nested/{Name}"`
		Slice  func(context.Context, *[]string) (int, error)             `rest:"GET /slice?v=%v"`
	}
	h, err := rest.Handler(nil, API{
		Query: func(_ context.Context, n *int) (string, error) {
			if n == nil {
				return "absent", nil
			}
			return fmt.Sprint(*n), nil
		},
		Path: func(_ context.Context, n *int) (string, error) { return fmt.Sprint(*n), nil },
		PStr: func(_ context.Context, s *string) (string, error) { return *s, nil },
		PTime: func(_ context.Context, s *time.Time) (string, error) {
			return s.UTC().Format(time.RFC3339), nil
		},
		Expand: func(_ context.Context, f *Filter) (string, error) { return fmt.Sprint(f.Limit), nil },
		Body:   func(_ context.Context, th *pointerThing) (string, error) { return th.Name, nil },
		Map: func(_ context.Context, th *pointerThing, n int) (string, error) {
			return fmt.Sprint(th.Name, n), nil
		},
		Nested: func(_ context.Context, th *pointerThing) (string, error) { return th.Name, nil },
		Slice:  func(_ context.Context, s *[]string) (int, error) { return len(*s), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, target, body, want string }{
		{"GET", "/query?limit=5", "", "5"},
		{"GET", "/query", "", "absent"},
		{"GET", "/path/5", "", "5"},
		{"GET", "/pstr?s=hi", "", "hi"},
		{"GET", "/ptime?t=2026-01-02T03:04:05Z", "", "2026-01-02T03:04:05Z"},
		{"GET", "/expand?limit=5", "", "5"},
		{"GET", "/expand", "", "0"},
		{"POST", "/body", `{"name":"x"}`, "x"},
		{"POST", "/body", `null`, ""},
		{"POST", "/map", `{"thing":{"name":"x"},"n":1}`, "x1"},
		{"POST", "/nested/abc", `{"name":"x"}`, "abc"},
		{"POST", "/nested/abc", `null`, "abc"},
		{"GET", "/slice?v=a&v=b", "", "2"},
	} {
		rec := serve(t, h, tc.method, tc.target, tc.body)
		if rec.Code != 200 || rec.Body.String() != tc.want {
			t.Errorf("%s %s body %q: %d %q, want %q", tc.method, tc.target, tc.body, rec.Code, rec.Body, tc.want)
		}
	}
}

func TestPointerQueryFields(t *testing.T) {
	type Filter struct {
		Limit *int       `json:"limit"`
		Name  *string    `json:"name"`
		Since *time.Time `json:"since"`
		Tags  []*string  `json:"tags"`
	}
	type API struct {
		api.Specification
		List func(context.Context, Filter) (string, error) `rest:"GET /list?%v"`
	}
	var query string
	h, err := rest.Handler(nil, API{
		List: func(_ context.Context, f Filter) (string, error) {
			if f.Limit == nil {
				return "absent", nil
			}
			out := fmt.Sprint(*f.Limit, " ", *f.Name)
			if f.Since != nil {
				out += " " + f.Since.UTC().Format(time.RFC3339)
			}
			for _, tag := range f.Tags {
				out += " " + *tag
			}
			return out, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	five, name, zero := 5, "bob", 0
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got, err := client.List(context.Background(), Filter{Limit: &five, Name: &name, Since: &when, Tags: []*string{&name}})
	if err != nil || got != "5 bob 2026-01-02T03:04:05Z bob" {
		t.Errorf("client.List: %q, %v (wire %q)", got, err, query)
	}
	if strings.Contains(query, "0x") {
		t.Errorf("the client sent a pointer address: %q", query)
	}
	got, err = client.List(context.Background(), Filter{Limit: &zero, Name: &name})
	if err != nil || got != "0 bob" {
		t.Errorf("a pointer to zero is a value: %q, %v (wire %q)", got, err, query)
	}
	if got, err = client.List(context.Background(), Filter{}); err != nil || got != "absent" {
		t.Errorf("nil pointers: %q, %v (wire %q)", got, err, query)
	}
}

func TestSlicePathParameters(t *testing.T) {
	type API struct {
		api.Specification
		Tags func(context.Context, []string) (string, error) `rest:"GET /tags/{tags=%v}"`
		Nums func(context.Context, []int) (int, error)       `rest:"GET /nums/{nums=%v}"`
		Blob func(context.Context, []byte) (string, error)   `rest:"GET /blob/{data=%v}"`
	}
	h, err := rest.Handler(nil, API{
		Tags: func(_ context.Context, s []string) (string, error) { return strings.Join(s, "|"), nil },
		Nums: func(_ context.Context, s []int) (int, error) { return len(s), nil },
		Blob: func(_ context.Context, s []byte) (string, error) { return string(s), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{
		"/tags/a":   "a",
		"/tags/a,b": "a|b",
		"/nums/1,2": "2",
		"/blob/a,b": "a,b",
	} {
		if rec := serve(t, h, "GET", target, ""); rec.Code != 200 || rec.Body.String() != want {
			t.Errorf("GET %s: %d %q, want %q", target, rec.Code, rec.Body, want)
		}
	}
	if rec := serve(t, h, "GET", "/nums/1,x", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("GET /nums/1,x: %d %q", rec.Code, rec.Body)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	if got, err := client.Tags(context.Background(), []string{"a", "b c"}); err != nil || got != "a|b c" {
		t.Errorf("client.Tags: %q, %v", got, err)
	}
	if got, err := client.Blob(context.Background(), []byte("hi/there")); err != nil || got != "hi/there" {
		t.Errorf("client.Blob: %q, %v", got, err)
	}
}

// requestCreds fills a hidden field from a header, on the pointer receiver.
type requestCreds struct {
	Name  string `json:"name"`
	Token string `json:"-"`
}

func (c *requestCreds) ReadHeadersHTTP(h http.Header) { c.Token = h.Get("X-Token") }

func TestArgumentsReadHeaders(t *testing.T) {
	type API struct {
		api.Specification
		Value   func(context.Context, requestCreds) (string, error)  `rest:"POST /value"`
		Pointer func(context.Context, *requestCreds) (string, error) `rest:"POST /pointer"`
		Query   func(context.Context, requestCreds) (string, error)  `rest:"GET /query?%v"`
		Absent  func(context.Context, *requestCreds) (string, error) `rest:"GET /absent?creds=%v"`
	}
	h, err := rest.Handler(nil, API{
		Value:   func(_ context.Context, c requestCreds) (string, error) { return c.Name + ":" + c.Token, nil },
		Pointer: func(_ context.Context, c *requestCreds) (string, error) { return c.Name + ":" + c.Token, nil },
		Query:   func(_ context.Context, c requestCreds) (string, error) { return c.Name + ":" + c.Token, nil },
		Absent: func(_ context.Context, c *requestCreds) (string, error) {
			if c == nil {
				return "nil", nil
			}
			return "set", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, target, body, want string }{
		{"POST", "/value", `{"name":"al"}`, "al:tok"},
		{"POST", "/pointer", `{"name":"al"}`, "al:tok"},
		{"GET", "/query?name=al", "", "al:tok"},
		{"GET", "/absent", "", "nil"},
	} {
		rec := serve(t, h, tc.method, tc.target, tc.body, "X-Token", "tok")
		if rec.Body.String() != tc.want {
			t.Errorf("%s %s: %d %q, want %q", tc.method, tc.target, rec.Code, rec.Body, tc.want)
		}
	}
}

func TestDestructuredNamesDoNotShareIndexes(t *testing.T) {
	type L4 struct {
		V string `json:"v"`
		W string `json:"w"`
	}
	type L3 struct {
		X string `json:"x"`
		Y string `json:"y"`
		Z string `json:"z"`
		D L4     `json:"d"`
	}
	type L2 struct {
		C L3 `json:"c"`
	}
	type L1 struct {
		B L2 `json:"b"`
	}
	type API struct {
		api.Specification
		Deep func(context.Context, L1) (string, error) `rest:"GET /deep?%v"`
	}
	h, err := rest.Handler(nil, API{
		Deep: func(_ context.Context, q L1) (string, error) {
			return strings.Join([]string{q.B.C.X, q.B.C.Y, q.B.C.Z, q.B.C.D.V, q.B.C.D.W}, ","), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := serve(t, h, "GET", "/deep?b.c.x=1&b.c.y=2&b.c.z=3&b.c.d.v=4&b.c.d.w=5", "")
	if rec.Body.String() != "1,2,3,4,5" {
		t.Errorf("nested fields mis-bound: %d %q", rec.Code, rec.Body)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	var q L1
	q.B.C = L3{"1", "2", "3", L4{"4", "5"}}
	if got, err := client.Deep(context.Background(), q); err != nil || got != "1,2,3,4,5" {
		t.Errorf("client.Deep: %q, %v", got, err)
	}
}

// listNode is a query struct that refers to itself.
type listNode struct {
	Name string    `json:"name"`
	Next *listNode `json:"next"`
}

func TestRecursiveQueryStruct(t *testing.T) {
	type API struct {
		api.Specification
		List func(context.Context, listNode) (string, error) `rest:"GET /list?%v"`
	}
	h, err := rest.Handler(nil, API{
		List: func(_ context.Context, n listNode) (string, error) { return n.Name, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "GET", "/list?name=a", ""); rec.Body.String() != "a" {
		t.Errorf("GET /list?name=a: %d %q", rec.Code, rec.Body)
	}
}

func TestMapExpansion(t *testing.T) {
	type API struct {
		api.Specification
		Q      func(context.Context, map[string]string) (string, error)      `rest:"GET /q?%v"`
		Counts func(context.Context, map[string]int) (int, error)            `rest:"GET /counts?%v"`
		Mixed  func(context.Context, string, map[string]string) (int, error) `rest:"GET /mixed?a=%v&%v"`
	}
	h, err := rest.Handler(nil, API{
		Q: func(_ context.Context, m map[string]string) (string, error) { return m["a"] + m["b"], nil },
		Counts: func(_ context.Context, m map[string]int) (int, error) {
			var total int
			for _, n := range m {
				total += n
			}
			return total, nil
		},
		Mixed: func(_ context.Context, a string, m map[string]string) (int, error) { return len(a) + len(m), nil },
	})
	if err != nil {
		t.Fatalf("documented map expansion refused: %v", err)
	}
	if rec := serve(t, h, "GET", "/q?a=1&b=2", ""); rec.Body.String() != "12" {
		t.Errorf("GET /q?a=1&b=2: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "GET", "/counts?a=1&b=2", ""); rec.Body.String() != "3" {
		t.Errorf("GET /counts?a=1&b=2: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "GET", "/counts?a=x", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("GET /counts?a=x: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "GET", "/mixed?a=xyz&b=2", ""); rec.Body.String() != "4" {
		t.Errorf("a named parameter is not also expanded: %d %q", rec.Code, rec.Body)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	if got, err := client.Q(context.Background(), map[string]string{"a": "1", "b": "2"}); err != nil || got != "12" {
		t.Errorf("client.Q: %q, %v", got, err)
	}
	if got, err := client.Counts(context.Background(), map[string]int{"a": 1, "b": 2}); err != nil || got != 3 {
		t.Errorf("client.Counts: %d, %v", got, err)
	}
	rec := serve(t, h, "GET", "/documentation", "", "Accept", "application/json")
	if rec.Code != 200 {
		t.Errorf("documentation of a map expansion: %d %q", rec.Code, rec.Body)
	}
}

func TestEmbeddedFieldInAPath(t *testing.T) {
	type Base struct {
		ID string
	}
	type Doc struct {
		Base
		Text string
	}
	type Ptr struct {
		*Base
		Text string
	}
	type Deep struct {
		Inner struct{ Base }
	}
	type API struct {
		api.Specification
		Put     func(context.Context, Doc) (string, error)  `rest:"PUT /docs/{ID}"`
		PutPtr  func(context.Context, Ptr) (string, error)  `rest:"PUT /ptrs/{ID}"`
		PutDeep func(context.Context, Deep) (string, error) `rest:"PUT /deep/{Inner.ID}"`
	}
	h, err := rest.Handler(nil, API{
		Put:     func(_ context.Context, d Doc) (string, error) { return d.ID + d.Text, nil },
		PutPtr:  func(_ context.Context, d Ptr) (string, error) { return d.ID + d.Text, nil },
		PutDeep: func(_ context.Context, d Deep) (string, error) { return d.Inner.ID, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "PUT", "/docs/1", `{"Text":"x"}`); rec.Body.String() != "1x" {
		t.Errorf("PUT /docs/1: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "PUT", "/ptrs/1", `{"Text":"x"}`); rec.Body.String() != "1x" {
		t.Errorf("PUT /ptrs/1: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "PUT", "/deep/1", `{}`); rec.Body.String() != "1" {
		t.Errorf("PUT /deep/1: %d %q", rec.Code, rec.Body)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	var doc Doc
	doc.ID, doc.Text = "1", "x"
	if got, err := client.Put(context.Background(), doc); err != nil || got != "1x" {
		t.Errorf("client.Put: %q, %v", got, err)
	}
}

func TestPathFieldOfAScalarIsAnError(t *testing.T) {
	type Thing struct {
		Name string
	}
	type API struct {
		api.Specification
		Get func(context.Context, Thing) error `rest:"PUT /x/{Name.Length}"`
	}
	defer func() {
		if p := recover(); p != nil {
			t.Errorf("Handler panicked: %v", p)
		}
	}()
	if _, err := rest.Handler(nil, API{}); err == nil {
		t.Errorf("a path into a string field was accepted")
	}
}

func TestStructPathParameterOnBodylessMethods(t *testing.T) {
	type Key struct {
		ID string
	}
	type thing struct {
		Name string `json:"name"`
	}
	type API struct {
		api.Specification
		Get  func(context.Context, Key) (string, error)   `rest:"GET /k/{ID}"`
		Del  func(context.Context, Key) error             `rest:"DELETE /k/{ID}"`
		Bulk func(context.Context, []string) (int, error) `rest:"DELETE /bulk"`
		Put  func(context.Context, thing) (string, error) `rest:"PUT /k/{name=$Name}"`
	}
	var deleted, sentType string
	h, err := rest.Handler(nil, API{
		Get: func(_ context.Context, k Key) (string, error) { return "got:" + k.ID, nil },
		Del: func(_ context.Context, k Key) error {
			deleted = k.ID
			return nil
		},
		Bulk: func(_ context.Context, ids []string) (int, error) { return len(ids), nil },
		Put:  func(_ context.Context, th thing) (string, error) { return th.Name, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "GET", "/k/5", ""); rec.Code != 200 || rec.Body.String() != "got:5" {
		t.Errorf("GET /k/5: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "DELETE", "/k/5", ""); rec.Code != http.StatusNoContent || deleted != "5" {
		t.Errorf("DELETE /k/5: %d %q, deleted %q", rec.Code, rec.Body, deleted)
	}
	var sentBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		sentBody, sentType = string(b), r.Method
		r.Body = io.NopCloser(strings.NewReader(sentBody))
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	if got, err := client.Get(context.Background(), Key{ID: "5"}); err != nil || got != "got:5" {
		t.Errorf("client.Get: %q, %v", got, err)
	}
	if sentBody != "" {
		t.Errorf("GET sent a body: %q", sentBody)
	}
	if err := client.Del(context.Background(), Key{ID: "6"}); err != nil || deleted != "6" {
		t.Errorf("client.Del: %v, deleted %q", err, deleted)
	}
	if sentBody != "" {
		t.Errorf("%s sent a body already carried by its path: %q", sentType, sentBody)
	}
	if n, err := client.Bulk(context.Background(), []string{"a", "b"}); err != nil || n != 2 {
		t.Errorf("client.Bulk: %d, %v (wire %q)", n, err, sentBody)
	}
	if got, err := client.Put(context.Background(), thing{Name: "x"}); err != nil || got != "x" {
		t.Errorf("client.Put: %q, %v", got, err)
	}
	rec := serve(t, h, "GET", "/documentation", "", "Accept", "application/json")
	var doc struct {
		Paths map[string]map[string]struct {
			RequestBody any `json:"requestBody"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Paths["/k/{ID}"]["get"].RequestBody != nil {
		t.Errorf("the documentation promises a body on a GET")
	}
}

func TestUnderscoreRules(t *testing.T) {
	type API struct {
		api.Specification
		Do   func(context.Context, string, string) (string, error) `rest:"POST /do (_trace,value)"`
		Out  func(context.Context) (string, string, error)         `rest:"GET /out _a,b"`
		Dash func(context.Context, string, string) (string, error) `rest:"POST /dash (x-trace,value)"`
	}
	var impl = API{
		Do:   func(_ context.Context, a, b string) (string, error) { return a + b, nil },
		Out:  func(context.Context) (string, string, error) { return "a", "b", nil },
		Dash: func(_ context.Context, a, b string) (string, error) { return a + b, nil },
	}
	defer func() {
		if p := recover(); p != nil {
			t.Errorf("Handler panicked: %v", p)
		}
	}()
	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "POST", "/do", `{"_trace":"t","value":"v"}`); rec.Body.String() != "tv" {
		t.Errorf("POST /do: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "POST", "/dash", `{"x-trace":"t","value":"v"}`); rec.Body.String() != "tv" {
		t.Errorf("POST /dash: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "GET", "/out", "", "Accept", "application/json"); !strings.Contains(rec.Body.String(), `"_a": "a"`) {
		t.Errorf("GET /out: %d %q", rec.Code, rec.Body)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	if a, b, err := client.Out(context.Background()); err != nil || a != "a" || b != "b" {
		t.Errorf("client.Out: %q %q %v", a, b, err)
	}
	if got, err := client.Do(context.Background(), "t", "v"); err != nil || got != "tv" {
		t.Errorf("client.Do: %q, %v", got, err)
	}
}

func TestBodyArgumentsPostAsAnArray(t *testing.T) {
	type API struct {
		api.Specification
		Pair func(context.Context, string, int) (string, error) `rest:"POST /pair"`
	}
	var wire string
	h, err := rest.Handler(nil, API{
		Pair: func(_ context.Context, a string, b int) (string, error) { return fmt.Sprint(a, b), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "POST", "/pair", `["a",1]`); rec.Code != 200 || rec.Body.String() != "a1" {
		t.Errorf("POST /pair: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "POST", "/pair", `["a"]`); rec.Code != http.StatusBadRequest {
		t.Errorf("too few elements: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "POST", "/pair", `["a",1,2]`); rec.Code != http.StatusBadRequest {
		t.Errorf("too many elements: %d %q", rec.Code, rec.Body)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		wire = string(b)
		r.Body = io.NopCloser(strings.NewReader(wire))
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	if got, err := client.Pair(context.Background(), "a", 1); err != nil || got != "a1" {
		t.Errorf("client.Pair: %q, %v (wire %q)", got, err, wire)
	}
}

func TestExpandedQueryErrorNamesTheParameter(t *testing.T) {
	type API struct {
		api.Specification
		Counts func(context.Context, map[string]int) (int, error) `rest:"GET /counts?%v"`
	}
	h, err := rest.Handler(nil, API{Counts: func(context.Context, map[string]int) (int, error) { return 0, nil }})
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "GET", "/counts?limit=x", ""); !strings.Contains(rec.Body.String(), "limit") {
		t.Errorf("%d %q", rec.Code, rec.Body)
	}
}
