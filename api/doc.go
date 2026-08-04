package api

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"runtime.link/api/test"
	"runtime.link/api/xray"
)

// Sampler reconstructs the downstream HTTP exchange for a function call from
// its arguments and return values: the "METHOD /path" line, the request body
// and the response body. It returns an empty url when the function does not
// correspond to an HTTP endpoint.
type Sampler func(fn Function, args, vals []reflect.Value) (url string, req, resp []byte, err error)

// sampler holds the registered [Sampler]. It is populated by the rest
// link-layer (package rest) via [RegisterSampler] to avoid an import cycle,
// since the HTTP reconstruction lives there.
var sampler Sampler

// RegisterSampler installs the [Sampler] used by [Documentation.Test] to attach
// the sampled request/response of each downstream call to the recorded
// [test.Event]. It is called once, from the rest link-layer's init.
func RegisterSampler(s Sampler) { sampler = s }

type literal string

type Documentation func(context.Context) (Examples, error)

func (fn Documentation) Example(ctx context.Context, name string) (Example, bool) {
	return fn.run(ctx, name, false)
}

func (fn Documentation) run(ctx context.Context, name string, test bool) (Example, bool) {
	if fn == nil {
		return Example{}, false
	}
	isolated, err := fn(ctx)
	if err != nil {
		return Example{
			Error: err,
		}, true
	}
	// The portion of name before the first ':' selects the environment when
	// the suite is an [Environments] map; the remainder is the method name. A
	// bare name (no ':') selects the "" environment, so a single-environment
	// suite behaves exactly as an ungrouped one.
	exampleName := name
	if envs, ok := isolated.(Environments); ok {
		env, method, found := strings.Cut(name, ":")
		if !found {
			env, method = "", name
		}
		sub, ok := envs[env]
		if !ok {
			return Example{}, false
		}
		isolated = sub
		exampleName = method
	} else if strings.Contains(name, ":") {
		parts := strings.SplitN(name, ":", 2)
		exampleName = parts[1]
	}
	method := reflect.ValueOf(isolated).MethodByName(exampleName)
	if !method.IsValid() {
		return Example{}, false
	}
	var (
		rtype  = reflect.TypeOf(isolated)
		rvalue = reflect.ValueOf(isolated)
	)
	example := isolated.example()
	// setup API capture
	for i := range rtype.Elem().NumField() {
		field := rtype.Elem().Field(i)
		if !field.IsExported() {
			continue
		}
		prefix := field.Tag.Get("rest")
		spec := StructureOf(rvalue.Elem().Field(i).Addr().Interface())
		spec.link([]string{field.Name})
		example.trace(spec, prefix)
	}
	example.Title = name
	writer, ok := method.Interface().(func(context.Context) error)
	if !ok {
		return Example{}, false
	}
	// Examples run against a fresh background context so documentation
	// rendering never inherits request-scoped state. Test runs, however,
	// must run against the caller's context so the xray recorder installed
	// by Test() captures internal API calls made by the implementation.
	runCtx := context.Background()
	if test {
		runCtx = ctx
	}
	func() {
		defer func() {
			if err := recover(); err != nil {
				example.Error = fmt.Errorf("panic %v %s", err, string(debug.Stack()))
				example.Panic = true
			}
		}()
		if err := writer(runCtx); err != nil {
			example.Error = err
		}
	}()
	return *example, true
}

// methods enumerates the func(context.Context) error methods of the
// documentation template, grouped by the file they are declared in, keeping
// only those methods for which keep returns true.
func (fn Documentation) methods(ctx context.Context, keep func(name string) bool) (map[string][]string, error) {
	if fn == nil {
		return nil, nil
	}
	template, err := fn(ctx)
	if err != nil {
		return nil, err
	}
	// Every environment in an [Environments] suite exposes the same set of
	// methods, so enumerate a single representative and return unprefixed
	// names. The environment is selected at run time, not listed per method.
	if envs, ok := template.(Environments); ok {
		var chosen Examples
		for _, key := range slices.Sorted(maps.Keys(envs)) {
			chosen = envs[key]
			break
		}
		if chosen == nil {
			return nil, nil
		}
		template = chosen
	}

	categories := make(map[string][]string)
	var rtype = reflect.TypeOf(template)
	var value = reflect.ValueOf(template)
	for i := range rtype.NumMethod() {
		method := rtype.Method(i)
		if _, ok := value.Method(i).Interface().(func(context.Context) error); !ok {
			continue
		}
		if keep != nil && !keep(method.Name) {
			continue
		}

		filename := extractFilenameFromMethod(method)
		categories[filename] = append(categories[filename], method.Name)
	}

	return categories, nil
}

// isTest reports whether a method name denotes a test rather than an example.
// Tests are distinguished by a "Test" prefix; everything else is an example.
func isTest(name string) bool {
	return strings.HasPrefix(name, "Test")
}

// Examples enumerates the example methods, grouped by the file they are
// declared in. Test methods (those prefixed with "Test") are excluded; use
// [Documentation.Tests] to enumerate those.
func (fn Documentation) Examples(ctx context.Context) (map[string][]string, error) {
	return fn.methods(ctx, func(name string) bool { return !isTest(name) })
}

// Tests enumerates the runnable methods, grouped by the file they are
// declared in. Every method is runnable: both examples (rendered as
// documentation) and dedicated test methods (prefixed with "Test"). The
// difference from [Documentation.Examples] is that Tests additionally
// includes the Test-prefixed methods, so the /testruns pages exercise the
// full suite.
func (fn Documentation) Tests(ctx context.Context) (map[string][]string, error) {
	return fn.methods(ctx, nil)
}

// Test runs the named test method and returns a [test.Execution] describing
// what happened: the story, the wall-clock duration, the ordered trace of
// calls (with their arguments and return values) and any error or panic.
// The boolean is false if no method with the given name exists.
func (fn Documentation) Test(ctx context.Context, name string) (test.Execution, bool) {
	start := time.Now()
	ctx = xray.NewContext(ctx)
	example, ok := fn.run(ctx, name, true)
	if !ok {
		return test.Execution{}, false
	}
	exec := test.Execution{
		Title: example.Title,
		Story: example.Story,
		Speed: time.Since(start),
		Panic: example.Panic,
		Ready: example.Ready,
	}
	if example.Error != nil {
		exec.Error = example.Error.Error()
	}
	// Collect top-level steps and internal (xray-recorded) calls into a
	// single flat list, each tagged with the monotonic sequence stamped at
	// its call-start, then order by that sequence so internal calls appear
	// inline between the top-level calls that made them.
	type ordered struct {
		seq   uint64
		event test.Event
	}
	var events []ordered
	for _, step := range example.Steps {
		event := test.Event{Note: step.Note}
		if step.Call != nil {
			event.Call = step.Call.Name
			event.Docs = eventDocs(restRoute(step.Call.Tags), step.Call.Docs)
			event.Args = encodeValues(step.Args)
			event.Vals = encodeValues(step.Vals)
			sampleInto(&event, *step.Call, step.Args, step.Vals)
		}
		events = append(events, ordered{seq: step.Seq, event: event})
	}
	for xray.ContextHas[xray.Call](ctx) {
		call := xray.ContextGet[xray.Call](ctx)
		event := test.Event{
			Call: call.Name,
			Docs: eventDocs(restRoute(call.Tags), DocumentationOf(reflect.StructField{Tag: call.Tags})),
			Args: encodeValues(call.Args),
			Vals: encodeValues(call.Vals),
		}
		// Reconstruct a minimal Function from the recorded call so the HTTP
		// exchange can be sampled the same way top-level steps are.
		if call.Func.IsValid() {
			sampleInto(&event, Function{
				Name: call.Name,
				Tags: call.Tags,
				Type: call.Func.Type(),
				Impl: call.Func,
			}, call.Args, call.Vals)
		}
		events = append(events, ordered{seq: call.Seq, event: event})
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].seq < events[j].seq
	})
	for _, e := range events {
		exec.Trace = append(exec.Trace, e.event)
	}
	return exec, true
}

// sampleInto attaches the sampled downstream HTTP exchange (url, request and
// response bodies) to the event, using the [Sampler] registered by the rest
// link-layer. It is a no-op when no sampler is registered or the function does
// not correspond to an HTTP endpoint, so a call that isn't served over REST
// simply carries no sampled exchange.
func sampleInto(event *test.Event, fn Function, args, vals []reflect.Value) {
	if sampler == nil {
		return
	}
	url, req, resp, err := sampler(fn, args, vals)
	if err != nil || url == "" {
		return
	}
	event.URL = url
	if len(req) > 0 {
		event.Req = jsonBody(req)
	}
	if len(resp) > 0 {
		event.Resp = jsonBody(resp)
	}
}

// jsonBody normalises a sampled request/response body into a value that is
// safe to store in a [json.RawMessage] field. Most sampled bodies are already
// JSON (the rest link-layer encodes them), but an fs.File / io.Reader body
// streams its raw bytes verbatim (an octet-stream upload, see the reader-body
// branch in the rest client writer), which is not valid JSON. Storing such
// bytes directly in a json.RawMessage produces a value that fails to
// re-marshal (json.RawMessage.MarshalJSON validates its contents), breaking
// any later encode of the enclosing [test.Execution]. When the body is not
// valid JSON we wrap it as a JSON string so the trace still records what was
// sent while remaining round-trippable.
func jsonBody(body []byte) json.RawMessage {
	if json.Valid(body) {
		return body
	}
	quoted, err := json.Marshal(string(body))
	if err != nil {
		return nil
	}
	return quoted
}

// History returns the [test.History] assigned to the underlying test
// implementation, or nil if the implementation does not embed a
// [TestingFramework] with an assigned History. The rest host uses this to
// decide whether the builtin /testrun/* endpoints should be served.
func (fn Documentation) History(ctx context.Context) test.History {
	if fn == nil {
		return nil
	}
	isolated, err := fn(ctx)
	if err != nil {
		return nil
	}
	with, ok := isolated.(WithHistory)
	if !ok {
		return nil
	}
	return with.history()
}

// Environments returns the sorted environment keys when the underlying suite is
// an [Environments] map, or nil otherwise (a single implicit "" environment).
// The rest host renders these as the /testruns environment selector.
func (fn Documentation) Environments(ctx context.Context) []string {
	if fn == nil {
		return nil
	}
	isolated, err := fn(ctx)
	if err != nil {
		return nil
	}
	envs, ok := isolated.(Environments)
	if !ok {
		return nil
	}
	return slices.Sorted(maps.Keys(envs))
}

// restRoute extracts the "METHOD /path" portion of a rest struct tag, dropping
// the trailing pattern/result rules (query params, body captures, result field
// names) so the trace can display the endpoint that was hit. It returns an
// empty string if there is no rest tag.
func restRoute(tags reflect.StructTag) string {
	rest := tags.Get("rest")
	if rest == "" {
		return ""
	}
	fields := strings.Fields(rest)
	switch len(fields) {
	case 0:
		return ""
	case 1:
		return fields[0]
	default:
		// fields[0] is the HTTP method, fields[1] is the path (which may
		// carry a ?query suffix); anything after is result/body rules.
		path := fields[1]
		if i := strings.IndexByte(path, '?'); i >= 0 {
			path = path[:i]
		}
		return fields[0] + " " + cleanPathCaptures(path)
	}
}

// cleanPathCaptures rewrites runtime.link path capture syntax "{name=%v}" into
// the plain "{name}" form for display, so a route reads as a conventional URL
// template rather than exposing the internal format verb.
func cleanPathCaptures(path string) string {
	for {
		open := strings.IndexByte(path, '{')
		if open < 0 {
			return path
		}
		close := strings.IndexByte(path[open:], '}')
		if close < 0 {
			return path
		}
		close += open
		segment := path[open+1 : close]
		if eq := strings.IndexByte(segment, '='); eq >= 0 {
			path = path[:open+1] + segment[:eq] + path[close:]
			continue
		}
		// No capture to rewrite in this segment; advance past it to avoid
		// looping forever on a plain "{name}".
		rest := cleanPathCaptures(path[close+1:])
		return path[:close+1] + rest
	}
}

// eventDocs combines the rest route and documentation string of a traced call
// into a single markdown blob for display beneath the call name. Either part
// may be empty.
func eventDocs(route, docs string) string {
	switch {
	case route != "" && docs != "":
		return "`" + route + "`\n\n" + docs
	case route != "":
		return "`" + route + "`"
	default:
		return docs
	}
}

// encodeValues renders a slice of reflected call arguments (or return values)
// as a JSON array, falling back to a string rendering for values that cannot
// be marshalled directly (channels, funcs, etc). A nil result is returned for
// an empty slice so the field is omitted from the record.
func encodeValues(values []reflect.Value) json.RawMessage {
	if len(values) == 0 {
		return nil
	}
	out := make([]json.RawMessage, 0, len(values))
	for _, v := range values {
		if !v.IsValid() {
			out = append(out, json.RawMessage("null"))
			continue
		}
		data, err := json.Marshal(v.Interface())
		if err != nil {
			data, _ = json.Marshal(fmt.Sprintf("%v", v.Interface()))
		}
		out = append(out, data)
	}
	data, err := json.Marshal(out)
	if err != nil {
		return nil
	}
	return data
}

type Example struct {
	Title string
	Tests string
	Story string
	Steps []Step
	Error error
	Panic bool

	// Ready reports whether the example called [TestingFramework.Ready],
	// marking it as promoted to the regression suite. Runners such as [Test]
	// only fail the build for examples that are Ready; a failure in an example
	// that has not been marked Ready is reported as skipped instead.
	Ready bool

	depth uint
	setup bool
}

type Step struct {
	Note string
	Call *Function
	Args []reflect.Value
	Vals []reflect.Value
	Time time.Time
	Seq  uint64

	Error  error
	Depth  uint
	Setup  bool
	Prefix string
}

type TestingFramework struct {
	eg Example

	History test.History
}

var _ WithExamples = (*Documentation)(nil)

type WithExamples interface {
	Example(context.Context, string) (Example, bool)
	Examples(context.Context) (map[string][]string, error)
}

type WithTests interface {
	Test(context.Context, string) (test.Execution, bool)
	Tests(context.Context) (map[string][]string, error)
	History(context.Context) test.History
	// Environments lists the selectable environments a test may be run
	// against, or nil when the suite is not an [Environments] map (in which
	// case there is a single implicit "" environment). The builtin /testruns
	// UI renders these as a selector and dispatches "env:TestName".
	Environments(context.Context) []string
}

type Examples interface {
	example() *Example
}

// Environments groups one [Examples] suite per named environment (for example
// "st", "dev", "sit-a"). It is itself an [Examples], so a [Documentation] may
// return it wherever a single suite is expected: the builtin /testruns UI then
// offers the environment as a selector and dispatches runs as "env:TestName".
//
// The "" key is the default environment; a suite returned directly (not via an
// Environments map) is treated as if it were Environments{"": suite}, so
// existing single-environment callers are unaffected.
type Environments map[string]Examples

// example satisfies [Examples]. The returned value is unused: [Documentation.run]
// resolves the selected environment's suite before reading its example.
func (Environments) example() *Example { return &Example{} }

// history satisfies [WithHistory] by returning the first assigned [test.History]
// found among the environments, so the /testruns endpoints are served whenever
// any environment carries one. Environments are expected to share a History.
func (e Environments) history() test.History {
	for _, key := range slices.Sorted(maps.Keys(e)) {
		if with, ok := e[key].(WithHistory); ok {
			if h := with.history(); h != nil {
				return h
			}
		}
	}
	return nil
}

// WithHistory is implemented by test [Examples] implementations that carry an
// assigned [test.History]. The rest host uses it to discover whether the
// builtin /testrun/* endpoints should be served for a given implementation.
type WithHistory interface {
	history() test.History
}

func (tdd *TestingFramework) example() *Example     { return &tdd.eg }
func (tdd *TestingFramework) history() test.History { return tdd.History }

// Ready marks the example as ready for regression testing. It should be called
// early in a test, before any API calls. Examples that call Ready are treated
// as part of the regression suite: runners such as [Test] fail on an error.
// Examples that do not call Ready are still executed but a failure
// is reported as skipped rather than failing the build, so work-in-progress
// examples can be committed without breaking `go test`.
func (tdd *TestingFramework) Ready() { tdd.eg.Ready = true }

func (tdd *TestingFramework) Story(description literal) { tdd.eg.Story = string(description) }
func (tdd *TestingFramework) Tests(description literal) { tdd.eg.Tests = string(description) }
func (tdd *TestingFramework) Setup(ctx context.Context, fn func(ctx context.Context) error) error {
	tdd.eg.setup = true
	defer func() {
		tdd.eg.setup = false
	}()
	if err := fn(ctx); err != nil {
		return err
	}
	return nil
}

func (tdd *TestingFramework) Guide(description literal) {
	now := time.Now()
	seq := xray.Sequence()
	// A note is meant to precede and merge with the following API call, so
	// reuse a trailing placeholder that has neither a note nor a recorded
	// call yet. Crucially, do NOT reuse a step that already holds a call
	// (e.g. one recorded during Setup), as that would clobber its ordering.
	if n := len(tdd.eg.Steps); n > 0 {
		last := &tdd.eg.Steps[n-1]
		if last.Note == "" && last.Call == nil {
			last.Note = string(description)
			last.Time = now
			last.Seq = seq
			return
		}
	}
	tdd.eg.Steps = append(tdd.eg.Steps, Step{Note: string(description), Time: now, Seq: seq})
}

func (eg *Example) trace(spec Structure, prefix ...string) {
	var pfx string
	if len(prefix) > 0 {
		pfx = prefix[0]
	}
	for i, old := range spec.Functions {
		old := old.Copy()
		fn := &spec.Functions[i]
		fn.Make(func(ctx context.Context, args []reflect.Value) (results []reflect.Value, err error) {
			eg.depth++
			defer func() {
				eg.depth--
			}()
			// Stamp a monotonic sequence before invoking so a flat trace
			// orders this top-level call ahead of any internal calls it
			// makes (which xray stamps at their own start).
			seq := xray.Sequence()
			start := time.Now()
			results, err = old.Call(ctx, args)
			if len(eg.Steps) == 0 {
				eg.Steps = append(eg.Steps, Step{})
			}
			step := &eg.Steps[len(eg.Steps)-1]
			if step.Call != nil {
				eg.Steps = append(eg.Steps, Step{})
				step = &eg.Steps[len(eg.Steps)-1]
			}
			step.Time = start
			step.Seq = seq
			step.Call = fn
			step.Args = args
			step.Vals = results
			step.Error = err
			step.Depth = eg.depth
			step.Setup = eg.setup
			step.Prefix = pfx
			return
		})
	}
	for _, section := range spec.Namespace {
		eg.trace(section, prefix...)
	}
}

func extractFilenameFromMethod(method reflect.Method) string {
	if method.Func.IsValid() {
		pc := method.Func.Pointer()
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			file, _ := fn.FileLine(pc)
			filename := filepath.Base(file)
			if strings.HasSuffix(filename, ".go") {
				filename = filename[:len(filename)-3]
			}
			return filename
		}
	}
	return "uncategorized"
}

func Test(t *testing.T, impl Documentation) {
	examples, err := impl.Examples(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, categoryExamples := range examples {
		for _, exampleName := range categoryExamples {
			t.Run(exampleName, func(t *testing.T) {
				example, _ := impl.Example(t.Context(), exampleName)
				if example.Error == nil {
					return
				}
				// Only examples that have opted into regression testing via
				// Ready() fail the build. Others are reported as skipped so
				// work-in-progress examples don't break `go test`.
				if example.Ready {
					t.Errorf("example %s failed %v", exampleName, example.Error)
				} else {
					t.Skipf("example %s not ready for regression testing, failed: %v", exampleName, example.Error)
				}
			})
		}
	}
	tests, err := impl.Tests(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, categoryTests := range tests {
		for _, testName := range categoryTests {
			t.Run(testName, func(t *testing.T) {
				exec, ok := impl.Test(t.Context(), testName)
				if !ok {
					t.Fatalf("test %s not found", testName)
				}
				if exec.Ready && exec.Error != "" {
					t.Errorf("test %s failed: %s", testName, exec.Error)
				}
			})
		}
	}
}
