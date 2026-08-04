package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"runtime.link/api"
	"runtime.link/api/test"
)

// yieldTestRuns mounts the builtin /testruns endpoints for a test
// implementation that carries an assigned [test.History]:
//
//	GET  /testruns          overview of the latest pass/fail status per test
//	GET  /testruns/{name}    show the recorded history for a test (does not run it)
//	POST /testruns/{name}    run the named test, record it, and redirect back
//
// Viewing a test never executes it; execution only happens on the POST, which
// is triggered by the "Run test" button on the detail page. This keeps side
// effects (real API calls against a live environment) deliberate.
//
// It returns false if the yield function requested that route iteration stop.
func yieldTestRuns(auth api.Auth[*http.Request], yield func(string, http.Handler) bool, tested api.WithTests) bool {
	if !yield("GET /testruns", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addCORS(auth, w, r, api.Function{})
		history := tested.History(r.Context())
		if history == nil {
			http.NotFound(w, r)
			return
		}
		summaries, err := history.Summary(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tests, _ := tested.Tests(r.Context())
		// Backfill the source file for each summary from the enumerated
		// tests (grouped by file), so the overview can group by file even
		// when the History implementation does not persist it.
		fillSummaryFrom(summaries, tests)
		writeTestRunHead(w)
		writeTestRunNav(w, tests, summaryStatus(summaries), "", "", "")
		w.Write([]byte("<main>"))
		defer w.Write([]byte("</main></body></html>"))
		fmt.Fprintf(w, "<h1>Test Runs</h1>")
		writeTestRunOverview(w, summaries)
	})) {
		return false
	}
	if !yield("GET /testruns/{name}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addCORS(auth, w, r, api.Function{})
		name := r.PathValue("name")
		history := tested.History(r.Context())
		if history == nil {
			http.NotFound(w, r)
			return
		}
		// Confirm the test exists without running it.
		tests, _ := tested.Tests(r.Context())
		if !testKnown(tests, name) {
			http.NotFound(w, r)
			return
		}
		// A ?env= selection scopes the history to a single environment. It is
		// carried by the POST/redirect so a run stays on the same environment's
		// page. Unknown values fall back to the default "" environment.
		envs := tested.Environments(r.Context())
		selected := r.URL.Query().Get("env")
		if selected != "" && !slices.Contains(envs, selected) {
			selected = ""
		}
		writeTestRunHead(w)
		// The detail page lives one path segment deeper than the
		// overview, so navigation links are prefixed with "../".
		summaries, _ := history.Summary(r.Context())
		navSuffix := ""
		if selected != "" {
			navSuffix = "?env=" + url.QueryEscape(selected)
		}
		writeTestRunNav(w, tests, summaryStatus(summaries), name, "../", navSuffix)
		w.Write([]byte("<main>"))
		defer w.Write([]byte("</main></body></html>"))
		fmt.Fprintf(w, "<h1>%s</h1>", html.EscapeString(formatExampleCategory(name)))
		writeTestRunButton(w, name, envs, selected)
		writeTestRunEnvScript(w, envs)

		// History is recorded under the environment-qualified title
		// ("env:TestName"); the bare name is the default "" environment.
		inspectName := name
		if selected != "" {
			inspectName = selected + ":" + name
		}
		runs, err := history.Inspect(r.Context(), inspectName)
		fmt.Fprintf(w, "<h2>History</h2>")
		var count int
		if err == nil && runs != nil {
			for past := range runs {
				writeTestRunHistoryEntry(w, past, count == 0)
				count++
			}
		}
		if count == 0 {
			fmt.Fprintf(w, "<p>No runs recorded yet. Run the test to record one.</p>")
		}
	})) {
		return false
	}
	if !yield("POST /testruns/{name}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addCORS(auth, w, r, api.Function{})
		name := r.PathValue("name")
		history := tested.History(r.Context())
		if history == nil {
			http.NotFound(w, r)
			return
		}
		// Build an absolute redirect target from the original request path so
		// the client lands back on this same detail page regardless of how the
		// docs handler is mounted (subpath-stripped at "/econnect" or hosted at
		// the root). A relative redirect ("./"+name) resolves against the
		// stripped path and 404s. Mirrors the "GET /" handler in host.go.
		target := r.RequestURI
		if target == "" {
			target = r.URL.Path
		}
		if i := strings.IndexByte(target, '?'); i >= 0 {
			target = target[:i]
		}
		// When the suite exposes environments, prefix the selected one so the
		// run dispatches against that environment ("env:TestName") and is
		// captured under that qualified title. An unknown or absent selection
		// falls back to the default "" environment.
		runName, redirect := name, target
		if env := r.FormValue("env"); env != "" && slices.Contains(tested.Environments(r.Context()), env) {
			runName = env + ":" + name
			redirect = target + "?env=" + url.QueryEscape(env)
		}
		exec, ok := tested.Test(r.Context(), runName)
		if !ok {
			http.NotFound(w, r)
			return
		}
		// Persist this execution so it appears in the test's history.
		// A capture failure is non-fatal.
		_ = history.Capture(r.Context(), exec)
		// Redirect back to the detail page so a refresh doesn't re-run
		// the test (POST/redirect/GET).
		http.Redirect(w, r, redirect, http.StatusSeeOther)
	})) {
		return false
	}
	return true
}

// testKnown reports whether name appears in the enumerated test categories.
func testKnown(tests map[string][]string, name string) bool {
	for _, names := range tests {
		if slices.Contains(names, name) {
			return true
		}
	}
	return false
}

// formatDuration renders a [time.Duration] for display in the test run UI.
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	return d.String()
}

// testRunStatus returns a badge glyph and label for an execution outcome.
func testRunStatus(errText string, panicked bool) (glyph, label string) {
	switch {
	case panicked:
		return "💥", "panic"
	case errText != "":
		return "❌", "fail"
	default:
		return "✅", "pass"
	}
}

// writeTestRunHead writes the shared HTML head + opening body tag for the
// test run pages, reusing the documentation stylesheet.
func writeTestRunHead(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte("<!DOCTYPE html>"))
	w.Write(docs_head)
	w.Write([]byte("<body>"))
}

// writeTestRunNav renders the left-hand navigation listing every test,
// grouped by category, highlighting the current selection. prefix is
// prepended to links so pages at different path depths resolve correctly.
// status maps a test name to its latest pass/fail glyph so each entry shows
// its outcome instead of the generic notepad icon. suffix is appended to each
// test link (e.g. "?env=st") so an environment selection stays sticky while
// navigating between tests.
func writeTestRunNav(w http.ResponseWriter, tests map[string][]string, status map[string]string, current, prefix, suffix string) {
	if len(tests) == 0 {
		return
	}
	w.Write([]byte("<nav>"))
	fmt.Fprintf(w, "<h2><a href=\"%sdocumentation\">← API Reference</a></h2>", prefix)
	fmt.Fprintf(w, "<h3><a href=\"%stestruns\">Test Runs</a></h3>", prefix)
	w.Write([]byte("<div class=\"examples-list\">"))
	categories := slices.Sorted(func(yield func(string) bool) {
		for k := range tests {
			if !yield(k) {
				return
			}
		}
	})
	for _, category := range categories {
		names := tests[category]
		if slices.Contains(names, current) {
			fmt.Fprintf(w, "<details class=\"example-category\" open>")
		} else {
			fmt.Fprintf(w, "<details class=\"example-category\">")
		}
		fmt.Fprintf(w, "<summary class=\"category-header\">%s</summary>", formatExampleCategory(category))
		fmt.Fprintf(w, "<div class=\"category-examples\">")
		for _, name := range names {
			title := formatExampleCategory(name)
			class := "example-link"
			if name == current {
				class = "example-link current-example"
			}
			state := status[name]
			if state == "" {
				state = "none"
			}
			fmt.Fprintf(w, "<a href=\"%stestruns/%s%s\" class=\"%s\" data-status=\"%s\">%s</a>", prefix, name, suffix, class, state, title)
		}
		fmt.Fprintf(w, "</div></details>")
	}
	w.Write([]byte("</div></nav>"))
}

// fillSummaryFrom populates the From (source file) of each summary that does
// not already carry one, looking the test name up in the file-grouped tests
// map. History implementations that persist From take precedence.
func fillSummaryFrom(summaries []test.Summary, tests map[string][]string) {
	if len(tests) == 0 {
		return
	}
	fileOf := make(map[string]string)
	for file, names := range tests {
		for _, name := range names {
			fileOf[name] = file
		}
	}
	for i := range summaries {
		if summaries[i].From == "" {
			summaries[i].From = fileOf[summaries[i].Name]
		}
	}
}

// summaryStatus maps each test name to a status keyword (pass, fail, todo)
// used as the data-status attribute on its nav link, driving the glyph shown
// by CSS. Tests without a summary are omitted, defaulting to "none".
func summaryStatus(summaries []test.Summary) map[string]string {
	status := make(map[string]string, len(summaries))
	for _, s := range summaries {
		switch {
		case s.Fail:
			status[s.Name] = "fail"
		case s.Pass:
			status[s.Name] = "pass"
		case s.Todo:
			status[s.Name] = "todo"
		}
	}
	return status
}

// writeTestRunOverview renders the pass/fail grid for all tests.
func writeTestRunOverview(w http.ResponseWriter, summaries []test.Summary) {
	if len(summaries) == 0 {
		fmt.Fprintf(w, "<p>No tests have been run yet. Select a test to run it.</p>")
		return
	}
	var passed, failed int
	for _, s := range summaries {
		if s.Pass {
			passed++
		}
		if s.Fail {
			failed++
		}
	}
	fmt.Fprintf(w, "<div class=\"markdown\">%d passing, %d failing, %d total.</div>", passed, failed, len(summaries))

	// Group the tests by the file they were declared in (Summary.From) so the
	// overview mirrors the source layout. Files are listed alphabetically and
	// tests keep their given order within each file.
	byFile := make(map[string][]test.Summary)
	for _, s := range summaries {
		from := s.From
		if from == "" {
			from = "uncategorized"
		}
		byFile[from] = append(byFile[from], s)
	}
	files := slices.Sorted(func(yield func(string) bool) {
		for k := range byFile {
			if !yield(k) {
				return
			}
		}
	})
	for _, file := range files {
		fmt.Fprintf(w, "<h2>%s</h2>", html.EscapeString(formatExampleCategory(file)))
		fmt.Fprintf(w, "<div class=\"examples-list\">")
		for _, s := range byFile[file] {
			var glyph string
			switch {
			case s.Fail:
				glyph = "❌"
			case s.Pass:
				glyph = "✅"
			default:
				glyph = "⚪"
			}
			fmt.Fprintf(w, "<div class=sample><pre>%s <a href=\"testruns/%s\">%s</a></pre></div>",
				glyph, html.EscapeString(s.Name), html.EscapeString(formatExampleCategory(s.Name)))
		}
		fmt.Fprintf(w, "</div>")
	}
}

// writeTestRunButton renders the form whose submission runs the test. It
// POSTs to the current detail URL, which executes the test, records it, and
// redirects back. When envs is non-empty an environment selector is included,
// preselecting selected, so the run targets a chosen live environment.
func writeTestRunButton(w http.ResponseWriter, name string, envs []string, selected string) {
	fmt.Fprintf(w, `<form method="POST" action="%s">`, html.EscapeString(name))
	if len(envs) > 0 {
		w.Write([]byte(`<select name="env" class="run-test-env">`))
		for _, env := range envs {
			sel := ""
			if env == selected {
				sel = " selected"
			}
			fmt.Fprintf(w, `<option value="%s"%s>%s</option>`, html.EscapeString(env), sel, html.EscapeString(env))
		}
		w.Write([]byte(`</select> `))
	}
	w.Write([]byte(`<button type="submit" class="run-test-button">▶ Run test</button></form>`))
}

// writeTestRunEnvScript emits a small script that remembers the selected
// environment in localStorage so it persists across tests and reloads. When
// the page is opened without an ?env= query (e.g. from the overview or a bare
// link) and a remembered environment is still offered, it redirects to the
// scoped URL so the history shown matches the selection. Changing the selector
// updates the remembered value. No script is emitted when there are no
// environments to choose between.
func writeTestRunEnvScript(w http.ResponseWriter, envs []string) {
	if len(envs) == 0 {
		return
	}
	const script = `<script>
(function () {
	var KEY = "testruns.env";
	var select = document.querySelector("select.run-test-env");
	if (!select) return;
	var params = new URLSearchParams(window.location.search);
	var hasEnv = params.has("env");
	var saved = null;
	try { saved = window.localStorage.getItem(KEY); } catch (e) {}
	// Only treat a saved value as valid if it is still an offered option, so a
	// removed environment can't trap the page in a redirect loop.
	var offered = saved && Array.prototype.some.call(select.options, function (o) { return o.value === saved; });
	if (!hasEnv && offered && saved !== "") {
		params.set("env", saved);
		window.location.replace(window.location.pathname + "?" + params.toString());
		return;
	}
	var urlEnv = params.get("env");
	var urlEnvOffered = urlEnv !== null && Array.prototype.some.call(select.options, function (o) { return o.value === urlEnv; });
	if (hasEnv && urlEnvOffered) {
		// The URL already scopes the env (server-rendered selection); remember
		// it so bare links pick up the latest choice.
		try { window.localStorage.setItem(KEY, urlEnv); } catch (e) {}
	} else if (!hasEnv && offered) {
		select.value = saved;
	}
	select.addEventListener("change", function () {
		try { window.localStorage.setItem(KEY, select.value); } catch (e) {}
	});
})();
</script>`
	w.Write([]byte(script))
}

// writeTestRunTrace renders the ordered trace of calls captured for an
// execution, showing the sampled downstream request/response where present and
// otherwise the raw arguments and return values.
func writeTestRunTrace(w http.ResponseWriter, trace []test.Event) {
	for _, event := range trace {
		if event.Note != "" {
			fmt.Fprintf(w, "<div class=\"markdown\">%s</div>", html.EscapeString(event.Note))
		}
		if event.Call == "" {
			continue
		}
		// Prefer the sampled HTTP line ("METHOD /path") as the heading when the
		// call was served over REST; fall back to the call name otherwise.
		heading := event.Call
		if event.URL != "" {
			heading = event.URL
		}
		fmt.Fprintf(w, "<div class=sample><pre>%s</pre>", html.EscapeString(heading))
		if event.Docs != "" {
			fmt.Fprintf(w, "<div class=\"markdown call-docs\">%s</div>", html.EscapeString(event.Docs))
		}
		// When the downstream HTTP exchange was sampled, show the request and
		// response bodies (matching the documentation example pages); otherwise
		// fall back to the raw Go arguments and return values.
		if event.URL != "" {
			if len(event.Req) > 0 {
				fmt.Fprintf(w, "<b>Request:</b><pre>%s</pre>", html.EscapeString(prettyJSON(event.Req)))
			}
			if len(event.Resp) > 0 {
				fmt.Fprintf(w, "<b>Response:</b><pre>%s</pre>", html.EscapeString(prettyJSON(event.Resp)))
			}
		} else {
			if len(event.Args) > 0 {
				fmt.Fprintf(w, "<b>Args:</b><pre>%s</pre>", html.EscapeString(prettyJSON(event.Args)))
			}
			if len(event.Vals) > 0 {
				fmt.Fprintf(w, "<b>Returns:</b><pre>%s</pre>", html.EscapeString(prettyJSON(event.Vals)))
			}
		}
		fmt.Fprintf(w, "</div>")
	}
}

// prettyJSON re-indents a JSON blob for display. Bodies are captured compactly
// (and older history rows were recorded before indentation was applied), so
// this formats them at render time. Content that is not valid JSON (e.g. XML)
// is returned unchanged.
func prettyJSON(raw []byte) string {
	var buf bytes.Buffer
	if json.Indent(&buf, raw, "", "\t") != nil {
		return string(raw)
	}
	return buf.String()
}

// writeTestRunHistoryEntry renders one recorded execution in a collapsible
// block. open controls whether the block is expanded by default; the caller
// expands the most recent run.
func writeTestRunHistoryEntry(w http.ResponseWriter, exec test.Execution, open bool) {
	glyph, label := testRunStatus(exec.Error, exec.Panic)
	if open {
		fmt.Fprintf(w, "<details open>")
	} else {
		fmt.Fprintf(w, "<details>")
	}
	fmt.Fprintf(w, "<summary>%s %s — %s</summary>", glyph, label, formatDuration(exec.Speed))
	if exec.Story != "" {
		fmt.Fprintf(w, "<div class=\"markdown\">%s</div>", html.EscapeString(exec.Story))
	}
	writeTestRunTrace(w, exec.Trace)
	if exec.Error != "" {
		fmt.Fprintf(w, "<b>Error:</b><pre>%s</pre>", html.EscapeString(exec.Error))
	}
	fmt.Fprintf(w, "</details>")
}
