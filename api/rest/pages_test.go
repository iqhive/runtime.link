package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	goscanner "go/scanner"
	"iter"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	textscanner "text/scanner"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
	"github.com/iqhive/runtime.link/api/test"
)

// pageSuite is a documentation suite that counts how often it is run.
type pageSuite struct {
	api.TestingFramework

	runs *atomic.Int32
	fail error
}

func (s *pageSuite) ExampleThing(context.Context) error {
	s.runs.Add(1)
	return s.fail
}

func (s *pageSuite) TestThing(context.Context) error {
	s.runs.Add(1)
	return nil
}

type pageDocs struct {
	api.Specification
	api.Documentation
}

// TestExamplesAreKeptByTheNamesThatAreListed checks that an example is run, and
// kept, for the names the documentation lists and for no other. Whatever stood
// before a colon was ignored when the example was looked up and was part of the
// key it was kept under, so a client could have it run, and kept, once for
// every spelling it thought of.
func TestExamplesAreKeptByTheNamesThatAreListed(t *testing.T) {
	var runs atomic.Int32
	h, err := rest.Handler(nil, &pageDocs{
		Documentation: func(context.Context) (api.Examples, error) { return &pageSuite{runs: &runs}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if rec := serve(t, h, "GET", "/examples/ExampleThing", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "<h1>Example Thing</h1>") {
			t.Fatalf("GET /examples/ExampleThing: %d %q", rec.Code, rec.Body)
		}
	}
	if runs.Load() != 1 {
		t.Fatalf("the example ran %d times for 3 requests", runs.Load())
	}
	names := []string{":ExampleThing", "st:ExampleThing", "a:b:ExampleThing", "ExampleThing:", "Nothing", "TestThing", "example_thing"}
	for i := range 50 {
		names = append(names, fmt.Sprintf("x%d:ExampleThing", i))
	}
	for _, name := range names {
		if rec := serve(t, h, "GET", "/examples/"+name, ""); rec.Code != http.StatusNotFound {
			t.Errorf("GET /examples/%s: %d, want 404", name, rec.Code)
		}
	}
	if runs.Load() != 1 {
		t.Errorf("names that are not listed ran the suite %d more times", runs.Load()-1)
	}
	// The documentation is made from the same examples, and does not run
	// them again either.
	for range 2 {
		if rec := serve(t, h, "GET", "/documentation", "", "Accept", "application/json"); rec.Code != 200 {
			t.Fatalf("GET /documentation: %d %q", rec.Code, rec.Body)
		}
	}
	if runs.Load() != 1 {
		t.Errorf("the documentation ran the example %d more times", runs.Load()-1)
	}
}

// TestExamplesOfAnEnvironmentAreKeptByItsName checks the one thing that may
// stand before the name of an example, which is an environment the suite has.
func TestExamplesOfAnEnvironmentAreKeptByItsName(t *testing.T) {
	var standard, staging atomic.Int32
	h, err := rest.Handler(nil, &pageDocs{
		Documentation: func(context.Context) (api.Examples, error) {
			return api.Environments{"": &pageSuite{runs: &standard}, "st": &pageSuite{runs: &staging}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		for _, name := range []string{"ExampleThing", "st:ExampleThing"} {
			if rec := serve(t, h, "GET", "/examples/"+name, ""); rec.Code != 200 {
				t.Fatalf("GET /examples/%s: %d", name, rec.Code)
			}
		}
	}
	for i := range 20 {
		name := fmt.Sprintf("x%d:ExampleThing", i)
		if rec := serve(t, h, "GET", "/examples/"+name, ""); rec.Code != http.StatusNotFound {
			t.Errorf("GET /examples/%s: %d, want 404", name, rec.Code)
		}
	}
	if standard.Load() != 1 || staging.Load() != 1 {
		t.Errorf("the example ran %d times as it is and %d times in st, want once each", standard.Load(), staging.Load())
	}
}

// TestAnExampleThatCouldNotBeSetUpIsAskedForAgain checks that a failure to set
// the documentation up is not kept as the outcome of the example: one request
// that had been cancelled, or one error that passes, was the answer to every
// request after it, and the example itself never ran.
func TestAnExampleThatCouldNotBeSetUpIsAskedForAgain(t *testing.T) {
	t.Run("a request that was cancelled", func(t *testing.T) {
		var runs atomic.Int32
		h, err := rest.Handler(nil, &pageDocs{
			Documentation: func(ctx context.Context) (api.Examples, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return &pageSuite{runs: &runs}, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		gone, cancel := context.WithCancel(context.Background())
		cancel()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/examples/ExampleThing", nil).WithContext(gone))
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("the cancelled request: %d %q", rec.Code, rec.Body)
		}
		if rec := serve(t, h, "GET", "/examples/ExampleThing", ""); rec.Code != 200 || runs.Load() != 1 {
			t.Errorf("the request after it: %d, the example ran %d times", rec.Code, runs.Load())
		}
	})
	t.Run("an error that passes", func(t *testing.T) {
		var runs, calls atomic.Int32
		h, err := rest.Handler(nil, &pageDocs{
			Documentation: func(context.Context) (api.Examples, error) {
				// The second call is the one that runs the example, the
				// first having listed it.
				if calls.Add(1) == 2 {
					return nil, errors.New("the database is restarting")
				}
				return &pageSuite{runs: &runs}, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		rec := serve(t, h, "GET", "/examples/ExampleThing", "")
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "the database is restarting") {
			t.Errorf("the request that failed: %d %q", rec.Code, rec.Body)
		}
		for range 2 {
			if rec := serve(t, h, "GET", "/examples/ExampleThing", ""); rec.Code != 200 {
				t.Errorf("a request after it: %d %q", rec.Code, rec.Body)
			}
		}
		if runs.Load() != 1 {
			t.Errorf("the example ran %d times, want 1", runs.Load())
		}
	})
	t.Run("an example that fails is what it is", func(t *testing.T) {
		var runs atomic.Int32
		h, err := rest.Handler(nil, &pageDocs{
			Documentation: func(context.Context) (api.Examples, error) {
				return &pageSuite{runs: &runs, fail: errors.New("the example is wrong")}, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		for range 3 {
			rec := serve(t, h, "GET", "/examples/ExampleThing", "")
			if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "the example is wrong") {
				t.Errorf("GET /examples/ExampleThing: %d %q", rec.Code, rec.Body)
			}
		}
		if runs.Load() != 1 {
			t.Errorf("the failing example ran %d times, want 1", runs.Load())
		}
	})
}

type vault struct {
	api.Specification

	Public func(context.Context) (string, error) `rest:"GET /public"`
	Secret func(context.Context) (string, error) `rest:"GET /admin/secret"`
	Audit  func(context.Context) (string, error) `rest:"GET /admin/audit"`

	steps []api.Step
}

func (v *vault) Examples(context.Context) (map[string][]string, error) {
	return map[string][]string{"all": {"Tour"}}, nil
}

func (v *vault) Example(_ context.Context, name string) (api.Example, bool) {
	return api.Example{Title: name, Steps: v.steps}, name == "Tour"
}

// newVault returns a vault whose example calls Public, then Secret, which calls
// Audit and Public in its turn, and then Audit.
func newVault() *vault {
	v := &vault{}
	functions := make(map[string]*api.Function)
	for _, fn := range api.StructureOf(v).Functions {
		functions[fn.Name] = &fn
	}
	step := func(name string, depth uint, result string) api.Step {
		return api.Step{Depth: depth, Call: functions[name], Vals: []reflect.Value{reflect.ValueOf(result)}}
	}
	v.steps = []api.Step{
		step("Public", 1, "first of Public"),
		{Note: "a note about what comes next"},
		step("Secret", 1, "result of Secret"),
		step("Audit", 2, "audit inside Secret"),
		step("Public", 3, "public inside Secret"),
		step("Audit", 1, "result of Audit"),
		step("Public", 1, "last of Public"),
	}
	return v
}

// TestExamplePageLeavesOutRefusedFunctions checks that the page of an example
// shows a caller no more than the documentation does: the route, the request
// and the result of a function that Authenticate refuses them were all on it.
func TestExamplePageLeavesOutRefusedFunctions(t *testing.T) {
	for _, c := range []struct {
		hidden        []string
		shown, unseen []string
	}{
		{
			hidden: []string{"Secret"},
			shown:  []string{"first of Public", "a note about what comes next", "result of Audit", "last of Public", "GET /admin/audit", "Example-&gt;&gt;vault API: Audit"},
			// What the refused function went on to call is part of what it does.
			unseen: []string{"/admin/secret", "result of Secret", "audit inside Secret", "public inside Secret", ": Secret"},
		},
		{
			hidden: []string{"Secret", "Audit"},
			shown:  []string{"first of Public", "last of Public", "GET /public"},
			unseen: []string{"/admin/", "result of Secret", "result of Audit", ": Secret", ": Audit"},
		},
		{
			shown: []string{"first of Public", "GET /admin/secret", "result of Secret", "result of Audit", ": Secret", ": Audit"},
		},
	} {
		h, err := rest.Handler(tokenAuth{hidden: c.hidden}, newVault())
		if err != nil {
			t.Fatal(err)
		}
		page := serve(t, h, "GET", "/examples/Tour", "", "X-Token", "ok")
		if page.Code != 200 {
			t.Fatalf("GET /examples/Tour: %d %q", page.Code, page.Body)
		}
		for _, text := range c.shown {
			if !strings.Contains(page.Body.String(), text) {
				t.Errorf("hiding %v: the page does not show %q:\n%s", c.hidden, text, page.Body)
			}
		}
		for _, text := range c.unseen {
			if strings.Contains(page.Body.String(), text) {
				t.Errorf("hiding %v: the page shows %q", c.hidden, text)
			}
		}
	}
	// With no Auth there is nobody to refuse anything.
	h, err := rest.Handler(nil, newVault())
	if err != nil {
		t.Fatal(err)
	}
	if page := serve(t, h, "GET", "/examples/Tour", ""); page.Code != 200 || !strings.Contains(page.Body.String(), "result of Secret") {
		t.Errorf("GET /examples/Tour without an Auth: %d %q", page.Code, page.Body)
	}
}

type vaultTests struct {
	api.Specification
	api.Documentation

	Public func(context.Context) (string, error) `rest:"GET /public"`
	Secret func(context.Context) (string, error) `rest:"GET /admin/secret"`
}

// TestTestRunPageLeavesOutRefusedCalls checks the history of a test as the page
// of an example is checked. A trace holds its calls by name, so a call is left
// out when a function of that name is refused the caller.
func TestTestRunPageLeavesOutRefusedCalls(t *testing.T) {
	var runs atomic.Int32
	history := &fakeHistory{}
	history.Capture(context.Background(), test.Execution{
		Title: "ExampleThing",
		Trace: []test.Event{
			{Call: "Public", URL: "GET /public", Resp: json.RawMessage(`"result of Public"`)},
			{Note: "a note of the test", Call: "Secret", URL: "GET /admin/secret", Docs: "docs of Secret",
				Req: json.RawMessage(`"request of Secret"`), Resp: json.RawMessage(`"result of Secret"`)},
			{Call: "Secret", Args: json.RawMessage(`["argument of Secret"]`), Vals: json.RawMessage(`["value of Secret"]`)},
			{Call: "Elsewhere", Vals: json.RawMessage(`["result of Elsewhere"]`)},
		},
	})
	impl := &vaultTests{
		Documentation: func(context.Context) (api.Examples, error) {
			return &pageSuite{TestingFramework: api.TestingFramework{History: history}, runs: &runs}, nil
		},
	}
	secrets := []string{"/admin/secret", "docs of Secret", "request of Secret", "result of Secret", "argument of Secret", "value of Secret"}
	open := []string{"GET /public", "result of Public", "a note of the test", "result of Elsewhere"}

	h, err := rest.Handler(tokenAuth{hidden: []string{"Secret"}}, impl)
	if err != nil {
		t.Fatal(err)
	}
	page := serve(t, h, "GET", "/testruns/ExampleThing", "", "X-Token", "ok", "Accept", "text/html")
	if page.Code != 200 {
		t.Fatalf("GET /testruns/ExampleThing: %d %q", page.Code, page.Body)
	}
	for _, text := range open {
		if !strings.Contains(page.Body.String(), text) {
			t.Errorf("the page does not show %q:\n%s", text, page.Body)
		}
	}
	for _, text := range secrets {
		if strings.Contains(page.Body.String(), text) {
			t.Errorf("the page shows %q of a function the caller is refused", text)
		}
	}

	h, err = rest.Handler(tokenAuth{}, impl)
	if err != nil {
		t.Fatal(err)
	}
	page = serve(t, h, "GET", "/testruns/ExampleThing", "", "X-Token", "ok", "Accept", "text/html")
	for _, text := range append(open, secrets...) {
		if !strings.Contains(page.Body.String(), text) {
			t.Errorf("a caller who is refused nothing is not shown %q", text)
		}
	}
	if runs.Load() != 0 {
		t.Errorf("showing the history ran the test %d times", runs.Load())
	}
}

// TestRunningATestNeedsAListedName checks that a test is run under the names its
// page exists under, and under no other: whatever stood before a colon was
// ignored when the test was looked up, so it ran, and was recorded, under any
// title a client made up.
func TestRunningATestNeedsAListedName(t *testing.T) {
	var runs atomic.Int32
	history := &fakeHistory{}
	h, err := rest.Handler(nil, &pageDocs{
		Documentation: func(context.Context) (api.Examples, error) {
			return &pageSuite{TestingFramework: api.TestingFramework{History: history}, runs: &runs}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"junk:ExampleThing", ":ExampleThing", "st:TestThing", "Nothing", "ExampleThing:"} {
		if rec := serve(t, h, "GET", "/testruns/"+name, ""); rec.Code != http.StatusNotFound {
			t.Errorf("GET /testruns/%s: %d, want 404", name, rec.Code)
		}
		if rec := serve(t, h, "POST", "/testruns/"+name, ""); rec.Code != http.StatusNotFound {
			t.Errorf("POST /testruns/%s: %d, want 404", name, rec.Code)
		}
	}
	if summaries, _ := history.Summary(context.Background()); runs.Load() != 0 || len(summaries) != 0 {
		t.Fatalf("names that are not listed ran the suite %d times and left %+v in the history", runs.Load(), summaries)
	}
	for _, name := range []string{"ExampleThing", "TestThing"} {
		if rec := serve(t, h, "POST", "/testruns/"+name, ""); rec.Code != http.StatusSeeOther {
			t.Errorf("POST /testruns/%s: %d, want 303", name, rec.Code)
		}
	}
	summaries, _ := history.Summary(context.Background())
	var titles []string
	for _, summary := range summaries {
		titles = append(titles, summary.Name)
	}
	if runs.Load() != 2 || len(titles) != 2 || !strings.Contains(strings.Join(titles, ","), "ExampleThing") || !strings.Contains(strings.Join(titles, ","), "TestThing") {
		t.Errorf("the listed tests ran %d times and are in the history as %q", runs.Load(), titles)
	}
}

type brokenHistory struct{}

func (brokenHistory) Summary(context.Context) ([]test.Summary, error) {
	return nil, errors.New("dial tcp: postgres://admin:hunter2@db.internal/tests refused")
}
func (brokenHistory) Inspect(context.Context, string) (iter.Seq[test.Execution], error) {
	return nil, nil
}
func (brokenHistory) Capture(context.Context, test.Execution) error { return nil }

type brokenTests struct {
	api.Specification

	Hello func(context.Context) (string, error) `rest:"GET /hello"`
}

func (brokenTests) Test(context.Context, string) (test.Execution, bool) {
	return test.Execution{}, false
}
func (brokenTests) Tests(context.Context) (map[string][]string, error) { return nil, nil }
func (brokenTests) History(context.Context) test.History               { return brokenHistory{} }
func (brokenTests) Environments(context.Context) []string              { return nil }

// TestTestRunsErrorIsRedacted checks that the error of a test history is the
// Auth's to rewrite before it is sent, as the error of a function is: it went
// out as it was, with the address of the database that it is.
func TestTestRunsErrorIsRedacted(t *testing.T) {
	h, err := rest.Handler(redactingAuth{}, brokenTests{})
	if err != nil {
		t.Fatal(err)
	}
	rec := serve(t, h, "GET", "/testruns", "", "Accept", "text/html")
	if rec.Code != http.StatusInternalServerError || strings.TrimSpace(rec.Body.String()) != "redacted" {
		t.Errorf("GET /testruns: %d %q, want 500 \"redacted\"", rec.Code, rec.Body)
	}
	// Without an Auth nothing rewrites an error, of a function or of this.
	h, err = rest.Handler(nil, brokenTests{})
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "GET", "/testruns", ""); rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "refused") {
		t.Errorf("GET /testruns without an Auth: %d %q", rec.Code, rec.Body)
	}
}

type identityKey struct{}

type identityAuth struct{}

func (identityAuth) Authenticate(ctx context.Context, r *http.Request, fn api.Function) (context.Context, error) {
	return context.WithValue(ctx, identityKey{}, "alice"), nil
}
func (identityAuth) Authorize(context.Context, *http.Request, api.Function, []reflect.Value) error {
	return nil
}
func (identityAuth) Redact(_ context.Context, err error) error { return err }

type identified struct {
	api.Specification

	Hello func(context.Context) (string, error) `rest:"GET /hello"`

	seen *[]any
}

func (d identified) Example(ctx context.Context, name string) (api.Example, bool) {
	*d.seen = append(*d.seen, ctx.Value(identityKey{}))
	return api.Example{Title: name}, name == "SayHello"
}

func (d identified) Examples(ctx context.Context) (map[string][]string, error) {
	*d.seen = append(*d.seen, ctx.Value(identityKey{}))
	if ctx.Value(identityKey{}) == nil {
		return nil, errors.New("who is asking?")
	}
	return map[string][]string{"examples_basic": {"SayHello"}}, nil
}

// TestPagesListExamplesForWhoIsAsking checks that every page hands the examples
// the context the Auth returned, which is where an identity is: the page of the
// documentation used the request's own, and came out without its examples.
func TestPagesListExamplesForWhoIsAsking(t *testing.T) {
	var seen []any
	h, err := rest.Handler(identityAuth{}, identified{seen: &seen})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ target, accept, want string }{
		{"/documentation", "text/html", "examples/SayHello"},
		{"/documentation", "application/json", `"openapi"`},
		{"/documentation", "application/javascript", "export function API"},
		{"/examples/SayHello", "text/html", "Say Hello"},
	} {
		rec := serve(t, h, "GET", c.target, "", "Accept", c.accept)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), c.want) {
			t.Errorf("GET %s as %s: %d, without %q in %q", c.target, c.accept, rec.Code, c.want, rec.Body)
		}
	}
	if len(seen) == 0 {
		t.Fatal("the examples were never asked for")
	}
	for _, identity := range seen {
		if identity != "alice" {
			t.Fatalf("the examples were asked for as %v", seen)
		}
	}
}

type oddMethods struct {
	api.Specification

	Hello func(context.Context) (string, error) `rest:"GET /hello"`
	Purge func(context.Context) error           `rest:"PURGE /cache"`
	Link  func(context.Context) error           `rest:"LINK /hello"`
}

// TestDocumentationHoldsFunctionsOfAnyMethod checks that a function with a
// method OpenAPI has no field for is documented like the rest. It was served,
// and was the end of the documentation of every function of its API.
func TestDocumentationHoldsFunctionsOfAnyMethod(t *testing.T) {
	h, err := rest.Handler(nil, oddMethods{
		Hello: func(context.Context) (string, error) { return "hi", nil },
		Purge: func(context.Context) error { return nil },
		Link:  func(context.Context) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := serve(t, h, "PURGE", "/cache", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("PURGE /cache: %d %q", rec.Code, rec.Body)
	}
	rec := serve(t, h, "GET", "/documentation", "", "Accept", "application/json")
	var doc struct {
		Paths map[string]struct {
			Get        *struct{ OperationID string } `json:"get"`
			Additional map[string]struct {
				OperationID string `json:"operationId"`
			} `json:"additionalOperations"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil || rec.Code != 200 {
		t.Fatalf("GET /documentation: %d %v %q", rec.Code, err, rec.Body)
	}
	if got := doc.Paths["/cache"].Additional["PURGE"].OperationID; got != "Purge" {
		t.Errorf("PURGE /cache is documented as %q in %s", got, rec.Body)
	}
	if hello := doc.Paths["/hello"]; hello.Get == nil || hello.Additional["LINK"].OperationID != "Link" || len(hello.Additional) != 1 {
		t.Errorf("/hello is documented as %+v", hello)
	}
	if rec := serve(t, h, "GET", "/documentation", "", "Accept", "application/javascript"); rec.Code != 200 {
		t.Errorf("the SDK: %d %q", rec.Code, rec.Body)
	}
}

type scanners struct {
	api.Specification

	Go    func(context.Context) (goscanner.Scanner, error)                     `rest:"GET /go"`
	Text  func(context.Context) (textscanner.Scanner, error)                   `rest:"GET /text"`
	Other func(context.Context, goscanner.Error) (textscanner.Position, error) `rest:"POST /other"`
}

// TestTypesOfOneNameHaveSchemasOfTheirOwn checks that two packages whose paths
// end alike are two namespaces to the documentation, the same two every time it
// is asked for. They were one, in which the second type of a name was
// documented, and typed by generated clients, with the fields of the first.
func TestTypesOfOneNameHaveSchemasOfTheirOwn(t *testing.T) {
	h, err := rest.Handler(nil, scanners{})
	if err != nil {
		t.Fatal(err)
	}
	var first string
	for range 5 {
		rec := serve(t, h, "GET", "/documentation", "", "Accept", "application/json")
		if first == "" {
			first = rec.Body.String()
		}
		if rec.Code != 200 || rec.Body.String() != first {
			t.Fatalf("GET /documentation: %d, and not what it was before:\n%s\n%s", rec.Code, first, rec.Body)
		}
	}
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Defs map[string]struct {
					Properties map[string]json.RawMessage `json:"properties"`
				} `json:"$defs"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal([]byte(first), &doc); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{
		`"#/components/schemas/scanner/$defs/Scanner"`,
		`"#/components/schemas/text.scanner/$defs/Scanner"`,
		// The other types of the two packages are where their package is.
		`"#/components/schemas/scanner/$defs/Error"`,
		`"#/components/schemas/text.scanner/$defs/Position"`,
	} {
		if !strings.Contains(first, ref) {
			t.Errorf("the documentation has no reference %s:\n%s", ref, first)
		}
	}
	properties := func(namespace, name string) []string {
		var names []string
		for property := range doc.Components.Schemas[namespace].Defs[name].Properties {
			names = append(names, property)
		}
		slices.Sort(names)
		return names
	}
	if got := properties("scanner", "Scanner"); !slices.Equal(got, []string{"ErrorCount"}) {
		t.Errorf("go/scanner.Scanner is documented with the fields %q", got)
	}
	if got := properties("text.scanner", "Scanner"); !slices.Contains(got, "Whitespace") || !slices.Contains(got, "Mode") {
		t.Errorf("text/scanner.Scanner is documented with the fields %q", got)
	}
	if got := properties("scanner", "Error"); !slices.Equal(got, []string{"Msg", "Pos"}) {
		t.Errorf("go/scanner.Error is documented with the fields %q", got)
	}
}
