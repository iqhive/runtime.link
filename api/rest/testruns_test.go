package rest_test

import (
	"context"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"runtime.link/api"
	"runtime.link/api/rest"
	"runtime.link/api/test"
)

// fakeHistory is an in-memory test.History for exercising the /testruns routes.
type fakeHistory struct {
	mu   sync.Mutex
	runs map[string][]test.Execution
}

func (h *fakeHistory) Capture(_ context.Context, result test.Execution) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.runs == nil {
		h.runs = make(map[string][]test.Execution)
	}
	h.runs[result.Title] = append(h.runs[result.Title], result)
	return nil
}

func (h *fakeHistory) Inspect(_ context.Context, title string) (iter.Seq[test.Execution], error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	runs := append([]test.Execution(nil), h.runs[title]...)
	return func(yield func(test.Execution) bool) {
		for i := len(runs) - 1; i >= 0; i-- {
			if !yield(runs[i]) {
				return
			}
		}
	}, nil
}

func (h *fakeHistory) Summary(_ context.Context) ([]test.Summary, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []test.Summary
	for title, runs := range h.runs {
		last := runs[len(runs)-1]
		out = append(out, test.Summary{Name: title, Pass: last.Error == ""})
	}
	return out, nil
}

// testDocs is the documentation implementation exercised by the tests below.
// runCount records how many times the test method actually executed, so the
// tests can assert that a GET does not run it.
type testDocs struct {
	api.TestingFramework

	runs *int
}

func (d *testDocs) ExampleThing(ctx context.Context) error {
	*d.runs++
	return nil
}

func TestTestRuns(t *testing.T) {
	var runs int
	history := &fakeHistory{}
	type Docs struct {
		api.Specification
		api.Documentation
	}
	impl := &Docs{
		Documentation: func(ctx context.Context) (api.Examples, error) {
			return &testDocs{
				TestingFramework: api.TestingFramework{History: history},
				runs:             &runs,
			}, nil
		},
	}
	handler, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}

	// GET the detail page: it must NOT run the test.
	get := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/testruns/ExampleThing", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	rec := get()
	if rec.Code != http.StatusOK {
		t.Fatalf("GET detail: status %d", rec.Code)
	}
	if runs != 0 {
		t.Fatalf("GET ran the test %d times, want 0", runs)
	}
	if !strings.Contains(rec.Body.String(), "Run test") {
		t.Errorf("GET detail missing run button")
	}
	if !strings.Contains(rec.Body.String(), "No runs recorded") {
		t.Errorf("GET detail should report no runs yet")
	}

	// POST runs the test once and redirects.
	req := httptest.NewRequest(http.MethodPost, "/testruns/ExampleThing", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST: status %d, want 303", rec.Code)
	}
	if runs != 1 {
		t.Fatalf("POST ran the test %d times, want 1", runs)
	}

	// After the POST, the detail page shows the recorded run (still without
	// re-running it).
	rec = get()
	if runs != 1 {
		t.Fatalf("second GET ran the test again (%d total)", runs)
	}
	if strings.Contains(rec.Body.String(), "No runs recorded") {
		t.Errorf("detail should show the recorded run after POST")
	}

	// Unknown tests 404 on GET without running anything.
	req = httptest.NewRequest(http.MethodGet, "/testruns/DoesNotExist", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET unknown test: status %d, want 404", rec.Code)
	}
}
