package rest

import (
	"bytes"
	"context"
	_ "embed"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"iter"
	"mime"
	"net/http"
	"path"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"runtime.link/api"
	"runtime.link/api/cors"
	http_api "runtime.link/api/internal/http"
	"runtime.link/api/internal/oas"
	"runtime.link/api/internal/rtags"
	"runtime.link/api/xray"
)

// requestFile adapts an incoming *http.Request body to an fs.File, so a handler
// can take an fs.File body parameter and read the uploaded bytes as they stream
// off the wire (no buffering into memory by the framework). Stat() synthesizes
// an fs.FileInfo from the request headers: the name from the Content-Disposition
// filename (falling back to the last path segment), and the size from
// Content-Length (-1 when unknown, e.g. chunked transfer).
type requestFile struct {
	body io.ReadCloser
	info requestFileInfo
}

func newRequestFile(r *http.Request) *requestFile {
	name := ""
	if cd := r.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			name = params["filename"]
		}
	}
	if name == "" {
		name = path.Base(r.URL.Path)
	}
	return &requestFile{
		body: r.Body,
		info: requestFileInfo{name: name, size: r.ContentLength},
	}
}

func (f *requestFile) Read(p []byte) (int, error) { return f.body.Read(p) }
func (f *requestFile) Close() error               { return f.body.Close() }
func (f *requestFile) Stat() (fs.FileInfo, error) { return f.info, nil }

// requestFileInfo is the fs.FileInfo synthesized for a streamed request body.
type requestFileInfo struct {
	name string
	size int64
}

func (i requestFileInfo) Name() string       { return i.name }
func (i requestFileInfo) Size() int64        { return i.size }
func (i requestFileInfo) Mode() fs.FileMode  { return 0 }
func (i requestFileInfo) ModTime() time.Time { return time.Time{} }
func (i requestFileInfo) IsDir() bool        { return false }
func (i requestFileInfo) Sys() any           { return nil }

// scanTypeOf resolves a textual parameter value into a concrete xyz.TypeOf[T]
// value for a tagged-union type selector (e.g. a filter field declared as
// xyz.TypeOf[Account]). Such a field is an interface, so it can't be scanned
// into directly; instead we recover the underlying union type from the
// interface's unexported value() T method, enumerate its cases via Values(),
// and match name against each case's Key()/String(). ok reports whether ref
// was a TypeOf interface that could be resolved.
func scanTypeOf(ref reflect.Value, name string) (ok bool, err error) {
	if ref.Kind() != reflect.Ptr || ref.Elem().Kind() != reflect.Interface {
		return false, nil
	}
	interfaceType := ref.Elem().Type()
	value, has := interfaceType.MethodByName("value")
	if !has || value.Type.NumOut() != 1 {
		return false, nil
	}
	variantType := value.Type.Out(0)
	values := reflect.New(variantType).Elem().MethodByName("Values")
	if !values.IsValid() || values.Type().NumIn() != 1 || values.Type().NumOut() != 1 {
		return false, nil
	}
	// Values(internal{}) returns a struct whose fields are the Case accessors,
	// each of which already implements the TypeOf[T] interface.
	cases := values.Call([]reflect.Value{reflect.New(values.Type().In(0)).Elem()})[0]
	for i := 0; i < cases.NumField(); i++ {
		field := cases.Field(i)
		typed, isCase := field.Interface().(interface {
			Key() (string, error)
			String() string
		})
		if !isCase {
			continue
		}
		if key, kerr := typed.Key(); kerr == nil && key == name || typed.String() == name {
			ref.Elem().Set(field)
			return true, nil
		}
	}
	return true, fmt.Errorf("no case named %q in %v", name, variantType)
}

func apiReferenceURL(fn api.Function) string {
	var categoryName string
	if len(fn.Path) == 0 {
		categoryName = "default"
	} else {
		categoryName = fn.Path[0]
	}

	return fmt.Sprintf("../#/%s/%s", categoryName, fn.Name)
}

var (
	//go:embed docs_head.html
	docs_head []byte
	//go:embed docs_body.html
	docs_body []byte
)

// fieldByIndex walks value along the given field index. It returns an invalid
// [reflect.Value] (rather than panicking) when it needs to descend into a value
// that is not a struct — this happens when an operation parsed from one
// function's rest tag is applied to a call whose argument shape differs (e.g.
// during trace sampling, where a scalar argument sits where a struct body was
// expected). Callers must check [reflect.Value.IsValid] before use.
func fieldByIndex(value reflect.Value, index []int) reflect.Value {
	if len(index) == 1 {
		if value.Kind() != reflect.Struct {
			return reflect.Value{}
		}
		return value.Field(index[0])
	}
	for i, x := range index {
		if i > 0 {
			if value.Kind() == reflect.Pointer && value.Type().Elem().Kind() == reflect.Struct {
				if value.IsNil() {
					if !value.CanSet() {
						return reflect.Zero(value.Type().Elem().FieldByIndex(index[i+1:]).Type)
					}
					value.Set(reflect.New(value.Type().Elem()))
				}
				value = value.Elem()
			}
		}
		if value.Kind() != reflect.Struct {
			return reflect.Value{}
		}
		value = value.Field(x)
	}
	return value
}

func isIteratorType(rtype reflect.Type) (isSeq bool, isSeq2 bool) {
	if rtype.Kind() != reflect.Func {
		return false, false
	}

	if rtype.NumIn() != 1 {
		return false, false
	}

	if rtype.NumOut() != 0 {
		return false, false
	}

	yieldType := rtype.In(0)
	if yieldType.Kind() != reflect.Func {
		return false, false
	}

	if yieldType.NumOut() != 1 || yieldType.Out(0).Kind() != reflect.Bool {
		return false, false
	}

	if yieldType.NumIn() == 1 {
		return true, false
	}

	if yieldType.NumIn() == 2 {
		return false, true
	}

	return false, false
}

type dataSource interface {
	Iterate(ctx context.Context, fn func(item any) bool) error
}

type channelSource struct {
	channel reflect.Value
	method  string
}

type iteratorSource struct {
	iterator   reflect.Value
	isSeq2     bool
	streamMode bool
}

func (cs channelSource) Iterate(ctx context.Context, fn func(item any) bool) error {
	cases := []reflect.SelectCase{
		{Dir: reflect.SelectRecv, Chan: cs.channel},
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())},
	}

	if cs.method == "GET" {
		chosen, value, ok := reflect.Select(cases)
		if chosen == 1 || !ok {
			return fmt.Errorf("no content")
		}
		fn(value.Interface())
		return nil
	} else if cs.method == "POST" {
		var lastValue reflect.Value
		for {
			chosen, value, ok := reflect.Select(cases)
			if chosen == 1 || !ok {
				if lastValue.IsValid() {
					fn(lastValue.Interface())
				} else {
					return fmt.Errorf("no content")
				}
				return nil
			}
			lastValue = value
		}
	}
	return fmt.Errorf("unsupported method: %s", cs.method)
}

func (is iteratorSource) Iterate(ctx context.Context, fn func(item any) bool) error {
	if is.isSeq2 {
		if is.streamMode {
			yieldFunc := reflect.MakeFunc(
				is.iterator.Type().In(0),
				func(args []reflect.Value) []reflect.Value {
					select {
					case <-ctx.Done():
						return []reflect.Value{reflect.ValueOf(false)}
					default:
						key := fmt.Sprintf("%v", args[0].Interface())
						value := args[1].Interface()
						obj := map[string]any{key: value}
						return []reflect.Value{reflect.ValueOf(fn(obj))}
					}
				},
			)
			is.iterator.Call([]reflect.Value{yieldFunc})
		} else {
			resultMap := make(map[string]any)
			yieldFunc := reflect.MakeFunc(
				is.iterator.Type().In(0),
				func(args []reflect.Value) []reflect.Value {
					select {
					case <-ctx.Done():
						return []reflect.Value{reflect.ValueOf(false)}
					default:
						key := fmt.Sprintf("%v", args[0].Interface())
						value := args[1].Interface()
						resultMap[key] = value
						return []reflect.Value{reflect.ValueOf(true)}
					}
				},
			)
			is.iterator.Call([]reflect.Value{yieldFunc})
			fn(resultMap)
		}
	} else {
		yieldFunc := reflect.MakeFunc(
			is.iterator.Type().In(0),
			func(args []reflect.Value) []reflect.Value {
				select {
				case <-ctx.Done():
					return []reflect.Value{reflect.ValueOf(false)}
				default:
					return []reflect.Value{reflect.ValueOf(fn(args[0].Interface()))}
				}
			},
		)
		is.iterator.Call([]reflect.Value{yieldFunc})
	}
	return nil
}

type streamWriter interface {
	WriteItem(ctx context.Context, item any) error
	Finalize(ctx context.Context, items []any) error
	SetupHeaders(w http.ResponseWriter)
}

type jsonStreamWriter struct {
	w     http.ResponseWriter
	items []any
}

type sseStreamWriter struct {
	w http.ResponseWriter
}

func (jsw *jsonStreamWriter) SetupHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func (jsw *jsonStreamWriter) WriteItem(ctx context.Context, item any) error {
	jsw.items = append(jsw.items, item)
	return nil
}

func (jsw *jsonStreamWriter) Finalize(ctx context.Context, items []any) error {
	if len(jsw.items) == 0 {
		jsw.w.WriteHeader(http.StatusNoContent)
		return nil
	}
	encoder := contentTypes["application/json"]
	return encoder.Encode(jsw.w, jsw.items)
}

func (ssw *sseStreamWriter) SetupHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (ssw *sseStreamWriter) WriteItem(ctx context.Context, item any) error {
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	fmt.Fprintf(ssw.w, "data: %s\n\n", data)
	if flusher, ok := ssw.w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func (ssw *sseStreamWriter) Finalize(ctx context.Context, items []any) error {
	return nil
}

func handleStreamingData(ctx context.Context, _ *http.Request, w http.ResponseWriter, source dataSource, writer streamWriter, fn api.Function, auth api.Auth[*http.Request]) {
	writer.SetupHeaders(w)

	var items []any
	err := source.Iterate(ctx, func(item any) bool {
		if err := writer.WriteItem(ctx, item); err != nil {
			handle(ctx, fn, auth, w, err)
			return false
		}
		items = append(items, item)
		return true
	})

	if err != nil {
		if err.Error() == "no content" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		handle(ctx, fn, auth, w, err)
		return
	}

	if err := writer.Finalize(ctx, items); err != nil {
		handle(ctx, fn, auth, w, err)
	}
}

func handleStreamingResult(ctx context.Context, r *http.Request, w http.ResponseWriter, result reflect.Value, fn api.Function, auth api.Auth[*http.Request]) {
	accept := r.Header.Get("Accept")

	var source dataSource

	if result.Kind() == reflect.Chan && result.Type().ChanDir() == reflect.RecvDir {
		if strings.Contains(accept, "application/json") && r.Method != "GET" && r.Method != "POST" {
			http.Error(w, "method not allowed for channel endpoints", http.StatusMethodNotAllowed)
			return
		}
		source = channelSource{channel: result, method: r.Method}
	} else if isSeq, isSeq2 := isIteratorType(result.Type()); isSeq || isSeq2 {
		streamMode := strings.Contains(accept, "text/event-stream")
		source = iteratorSource{iterator: result, isSeq2: isSeq2, streamMode: streamMode}
	} else {
		return
	}

	var writer streamWriter
	if strings.Contains(accept, "text/event-stream") {
		if r.Method != "GET" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writer = &sseStreamWriter{w: w}
	} else if strings.Contains(accept, "application/json") {
		writer = &jsonStreamWriter{w: w}
	} else {
		if result.Kind() == reflect.Chan {
			websocketServeHTTP(ctx, r, w, result, reflect.Value{})
			return
		} else {
			writer = &jsonStreamWriter{w: w}
		}
	}

	handleStreamingData(ctx, r, w, source, writer, fn, auth)
}

// Handlers can be used to integrete with different HTTP routers, it returns an iterator over the endpoints in the
// API, with a path pattern of the form fmt.Sprintf("GET /path/"+param_format, param) so that parameter format can
// be transformed for compatibility with different routers. The remainder_format is used to format the path in the
// case that an asterisk is used at the end of a path to capture the remainder of the path (including slashes).
func Handlers(auth api.Auth[*http.Request], impl any, param_format, remainder_format string) (iter.Seq2[string, http.Handler], error) {
	spec, err := specificationOf(api.StructureOf(impl))
	if err != nil {
		return nil, xray.New(err)
	}
	var (
		exampleCache   = make(map[string]api.Example)
		exampleCacheMu sync.Mutex
	)
	cachedExample := func(documented api.WithExamples, ctx context.Context, name string) (api.Example, bool) {
		exampleCacheMu.Lock()
		if eg, ok := exampleCache[name]; ok {
			exampleCacheMu.Unlock()
			return eg, true
		}
		exampleCacheMu.Unlock()
		eg, ok := documented.Example(ctx, name)
		if !ok {
			return eg, false
		}
		exampleCacheMu.Lock()
		exampleCache[name] = eg
		exampleCacheMu.Unlock()
		return eg, true
	}
	return func(yield func(string, http.Handler) bool) {
		var err error
		if param_format != "{%s}" {
			old_yield := yield
			yield = func(pattern string, handler http.Handler) bool {
				method, path, _ := strings.Cut(pattern, " ")
				split := strings.Split(path, "/")
				for i, part := range split {
					if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
						format := param_format
						if strings.HasSuffix(part, "*") {
							format = remainder_format
						}
						split[i] = fmt.Sprintf(format, part[1:len(part)-1])
					}
				}
				path = strings.Join(split, "/")
				return old_yield(fmt.Sprintf("%s %s", method, path), handler)
			}
		}
		// docsHandler serves the OpenAPI spec (JSON), the generated SDK
		// (JavaScript) and the Swagger UI / examples nav (HTML). It is
		// mounted at both "/" and "/documentation"; "/" responds with a
		// 302 redirect to "./documentation" for HTML/swagger requests so that
		// the relative example links emitted by handleDocs (e.g.
		// "./examples/Ordering") resolve to "<prefix>/examples/Ordering"
		// regardless of the parent mount point.
		docsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if auth != nil {
				addCORS(auth, w, r, api.Function{})
				ctx, err = auth.Authenticate(r.Context(), r, api.Function{})
				if err != nil {
					if strings.Contains(r.Header.Get("Accept"), "text/html") || strings.Contains(r.Header.Get("Accept"), "application/schema+json") {
						w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
						http.Error(w, "Unauthorized", http.StatusUnauthorized)
						return
					}
					handle(r.Context(), api.Function{}, auth, w, err)
					return
				}
			}
			if strings.Contains(r.Header.Get("Accept"), "application/json") {
				w.Header().Set("Content-Type", "application/json")
				structure := exercisedStructure(ctx, impl, spec.Structure, cachedExample)
				docs, err := oasDocumentOf(ctx, auth, r, structure)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if docs.Information.Title == "" {
					rtype := reflect.TypeOf(impl)
					docs.Information.Title = oas.Readable(path.Base(rtype.PkgPath()) + " " + rtype.Name())
				}
				if basePath := r.Header.Get("X-Base-Path"); basePath != "" {
					var servers []oas.Server
					for _, p := range strings.Split(basePath, ",") {
						servers = append(servers, oas.Server{URL: oas.URL(p)})
					}
					docs.Servers = servers
				}
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				enc.Encode(docs)
				return
			}
			if strings.Contains(r.Header.Get("Accept"), "application/javascript") {
				w.Header().Set("Content-Type", "application/javascript")
				structure := exercisedStructure(ctx, impl, spec.Structure, cachedExample)
				docs, err := oasDocumentOf(ctx, auth, r, structure)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				code, err := sdkFor(docs)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Write(code)
				return
			}
			if strings.Contains(r.Header.Get("Accept"), "text/html") {
				w.Header().Set("Content-Type", "text/html")
				handleDocs(r, w, func(err error) error {
					return auth.Redact(r.Context(), err)
				}, impl)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		})
		if !yield("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Build an absolute redirect target so the client lands on
			// "<original-path>/documentation" regardless of how the
			// docs handler is mounted (subpath-stripped at "/echannel"
			// or hosted at the root). Relative redirects ("./documentation")
			// fail when the original request had no trailing slash
			// because the resolver drops the last non-slash segment.
			target := r.RequestURI
			if target == "" {
				target = r.URL.Path
			}
			// Strip query string; we don't propagate it to /documentation.
			if i := strings.IndexByte(target, '?'); i >= 0 {
				target = target[:i]
			}
			target = strings.TrimSuffix(target, "/") + "/documentation"
			http.Redirect(w, r, target, http.StatusFound)
		})) {
			return
		}
		if !yield("GET /documentation", docsHandler) {
			return
		}
		if documented, ok := impl.(api.WithExamples); ok {
			if !yield("GET /examples/{name}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				addCORS(auth, w, r, api.Function{})
				name := r.PathValue("name")
				example, ok := cachedExample(documented, r.Context(), name)
				if !ok {
					http.NotFound(w, r)
					return
				}
				// Surface example failures (test errors or panics) as
				// HTTP 500 so AOT bake pipelines can fail the build
				// while still receiving the rendered HTML in the
				// response body for inspection.
				if example.Error != nil || example.Panic {
					w.WriteHeader(http.StatusInternalServerError)
				}
				w.Write([]byte("<!DOCTYPE html>"))
				w.Write(docs_head)
				w.Write([]byte("<body>"))
				examples, err := documented.Examples(r.Context())
				if err == nil {
					w.Write([]byte("<nav>"))
					fmt.Fprintf(w, "<h2><a href=\"../documentation\">← API Reference</a></h2>")
					w.Write([]byte("<h3>Examples</h3>"))

					w.Write([]byte("<div class=\"examples-list\">"))
					categories := slices.Sorted(func(yield func(string) bool) {
						for k := range examples {
							if !yield(k) {
								return
							}
						}
					})
					for _, category := range categories {
						categoryExamples := examples[category]
						isCurrentCategory := slices.Contains(categoryExamples, name)
						if isCurrentCategory {
							fmt.Fprintf(w, "<details class=\"example-category\" open>")
						} else {
							fmt.Fprintf(w, "<details class=\"example-category\">")
						}
						fmt.Fprintf(w, "<summary class=\"category-header\">%s</summary>", formatExampleCategory(category))
						fmt.Fprintf(w, "<div class=\"category-examples\">")
						for _, exampleName := range categoryExamples {
							title := formatExampleCategory(exampleName)
							if exampleName == name {
								fmt.Fprintf(w, "<a href=\"%v\" class=\"example-link current-example\">%s</a>", exampleName, title)
							} else {
								fmt.Fprintf(w, "<a href=\"%v\" class=\"example-link\">%s</a>", exampleName, title)
							}
						}
						fmt.Fprintf(w, "</div></details>")
					}
					w.Write([]byte("</div></nav>"))
				}
				w.Write([]byte("<main>"))
				defer w.Write([]byte("</main></body></html>"))
				header := "#" + example.Title + " " + example.Story
				if example.Error == nil {
					header = "✅ " + header
				} else {
					header = "❌ " + header
				}
				fmt.Fprintf(w, "<h1>%v</h1>", html.EscapeString(formatExampleCategory(example.Title)))
				if example.Story != "" {
					fmt.Fprintf(w, "<div class=\"markdown\">%v</div>", html.EscapeString(example.Story))
				}
				if example.Tests != "" {
					fmt.Fprintf(w, "<div class=\"markdown\">Tests %s</div>", html.EscapeString(example.Tests))
				}
				var mermaid bytes.Buffer
				fmt.Fprintf(&mermaid, "sequenceDiagram\n")
				var showable = false
				var depth uint = 1
				var stack = []string{"Example"}
				var space string = "Example"
				for _, step := range example.Steps {
					if step.Setup {
						continue
					}
					if step.Call != nil {
						if step.Depth > depth {
							stack = append(stack, space)
						}
						if step.Depth < depth {
							stack = stack[:step.Depth]
						}
						showable = true
						fmt.Fprintf(&mermaid, "%s->>%s: %s\n",
							stack[len(stack)-1], step.Call.Root.Name+" API", step.Call.Name)
						space = step.Call.Root.Name + " API"
						depth = step.Depth
					}
				}
				if showable {
					fmt.Fprintf(w, "<details><summary>Sequence Diagram</summary>")
					fmt.Fprintf(w, `<pre class="mermaid">%s</pre>`, html.EscapeString(mermaid.String()))
					fmt.Fprintf(w, "</details>")
				}
				for _, step := range example.Steps {
					if step.Note != "" {
						fmt.Fprintf(w, "<div class=\"markdown\">%s</div>", html.EscapeString(step.Note))
					}
					if step.Depth > 1 || step.Setup {
						continue
					}
					if step.Call != nil {
						url, req, resp, err := sample(*step.Call, step.Args, step.Vals)
						if err != nil {
							fmt.Fprintf(w, "<b>Error:</b>")
							fmt.Fprintf(w, "<pre>%s</pre>", err)
							continue
						}
						if step.Prefix != "" {
							if method, path, ok := strings.Cut(url, " "); ok {
								url = method + " " + step.Prefix + path
							}
						}
						apiRefURL := apiReferenceURL(*step.Call)
						var queryHint string
						if method, _, ok := strings.Cut(url, " "); ok && method == "QUERY" {
							queryHint = ` <span class="query-hint" title="If your infrastructure does not support the HTTP QUERY method, you can send a POST request with the header X-HTTP-Method-Override: QUERY instead.">&#x3f;</span>`
						}
						fmt.Fprintf(w, "<div class=sample><pre>%v%s <a href=\"%s\" target=\"_blank\" class=\"api-ref-link\">📖 View in API Reference</a></pre>", url, queryHint, apiRefURL)
						if len(req) > 0 {
							fmt.Fprintf(w, "<b>Request:</b>")
							fmt.Fprintf(w, "<pre>%s</pre>", req)
						}
						if len(resp) > 0 {
							fmt.Fprintf(w, "<b>Response:</b>")
							fmt.Fprintf(w, "<pre>%s</pre>", resp)
						}
						fmt.Fprintf(w, "</div>")
					}
				}
				if err := example.Error; err != nil {
					var value any = err
					if auth != nil {
						value = auth.Redact(r.Context(), err)
					}
					pretty, err := json.MarshalIndent(value, "", "    ")
					if err == nil && !bytes.Equal(pretty, []byte("{}")) {
						value = string(pretty)
					}
					fmt.Fprintf(w, "<details><summary>Error</summary><pre>%s</pre></details>", html.EscapeString(fmt.Sprint(value)))
				}
			})) {
				return
			}
		}
		if tested, ok := impl.(api.WithTests); ok && tested.History(context.Background()) != nil {
			if !yieldTestRuns(auth, yield, tested) {
				return
			}
		}
		attach(auth, yield, spec)
	}, nil
}

// Handler returns a HTTP handler that serves supported API types.
func Handler(auth api.Auth[*http.Request], impl any) (http.Handler, error) {
	var router = new(mux)
	notfound := http.NotFoundHandler()
	router.for404 = &notfound
	handlers, err := Handlers(auth, impl, "{%s}", "{%s}")
	if err != nil {
		return nil, xray.New(err)
	}
	for pattern, handler := range handlers {
		router.Handle(pattern, handler)
	}
	return router, nil
}

func handle(ctx context.Context, fn api.Function, auth api.Auth[*http.Request], rw http.ResponseWriter, err error) {
	if writer, ok := err.(http_api.HeaderWriter); ok {
		writer.WriteHeadersHTTP(rw.Header())
	}
	if auth != nil {
		err = auth.Redact(ctx, err)
	}
	var (
		status int = http.StatusInternalServerError
	)
	var (
		message = err.Error()
	)
	switch v := err.(type) {
	case http_api.Error:
		status = v.StatusHTTP()
		if status == 0 {
			status = http.StatusInternalServerError
		}
	default:
		if errors.Is(err, http_api.ErrNotImplemented) {
			status = http.StatusNotImplemented
			message = "not implemented"
		}
	}
	if contentTyped, ok := err.(interface {
		ContentTypeHTTP() string
	}); ok {
		switch contentTyped.ContentTypeHTTP() {
		case "application/json":
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(status)
			enc := json.NewEncoder(rw)
			enc.SetIndent("", "  ")
			enc.Encode(err)
			return
		}
	}
	for _, scenario := range fn.Root.Scenarios {
		if scenario.Test(err) {
			code, _ := strconv.Atoi(scenario.Tags.Get("http"))
			if code != 0 {
				status = code
			}
			if scenario.Text != "" {
				message = scenario.Text
			}
			break
		}
	}
	http.Error(rw, message, status)
}

func addCORS(auth api.Auth[*http.Request], w http.ResponseWriter, r *http.Request, fn api.Function) {
	if auth == nil {
		return
	}
	if auth, ok := auth.(cors.Authenticator); ok {
		control := auth.CrossOriginResourceSharing(r, fn)
		if control.AllowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", control.AllowOrigin)
		}
		if control.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", strconv.FormatBool(control.AllowCredentials))
		}
		if control.AllowHeaders != "" {
			w.Header().Set("Access-Control-Allow-Headers", control.AllowHeaders)
		}
		if control.AllowMethods != "" {
			w.Header().Set("Access-Control-Allow-Methods", control.AllowMethods)
		}
		if control.ExposeHeaders != "" {
			w.Header().Set("Access-Control-Expose-Headers", control.ExposeHeaders)
		}
		if control.MaxAge != 0 {
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(control.MaxAge))
		}
	}
}

func attach(auth api.Auth[*http.Request], yield func(string, http.Handler) bool, spec specification) {
	for path, resource := range spec.Resources {
		var hasGet = false
		var hasOptions = false
		for method, operation := range resource.Operations {
			var (
				op   = operation
				fn   = op.Function
				path = rtags.CleanupPattern(path)

				resultRules = rtags.ResultRulesOf(string(fn.Tags.Get("rest")))

				responseNeedsMapping  = len(resultRules) > 0
				argumentsNeedsMapping = len(rtags.ArgumentRulesOf(string(fn.Tags.Get("rest")))) > 0
			)
			if method == "GET" {
				if !yield("OPTIONS "+path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					addCORS(auth, w, r, fn)
					w.WriteHeader(200)
				})) {
					return
				}
			}
			if method == "GET" {
				hasGet = true
			}
			if method == "OPTIONS" {
				hasOptions = true
			}
			if !yield(string(method)+" "+path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				addCORS(auth, w, r, fn)
				var (
					ctx = r.Context()
					err error
				)
				var closeBody bool = r.Body != nil
				defer func() {
					if closeBody {
						r.Body.Close()
					}
				}()
				if auth != nil {
					ctx, err = auth.Authenticate(r.Context(), r, fn)
					if err != nil {
						handle(ctx, fn, auth, w, err)
						return
					}
				}
				if op.DefaultContentType != "text/html" && method == "GET" && strings.Contains(r.Header.Get("Accept"), "text/html") || strings.Contains(r.Header.Get("Accept"), "application/schema+json") {
					formHandler{res: resource}.ServeHTTP(w, r)
					return
				}
				ctype, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
				if ctype == "" {
					ctype = string(op.DefaultContentType)
				}
				if ctype == "" {
					ctype = "application/json"
				}
				decoder, decoderOk := contentTypes[ctype]
				var args = make([]reflect.Value, fn.NumIn())
				for i := range args {
					args[i] = reflect.New(fn.In(i)).Elem()
				}
				var mapped any
				var mappedCount int
				if argumentsNeedsMapping {
					mapped = reflect.New(op.argMappingType).Interface()
					if !decoderOk {
						http.Error(w, http.StatusText(http.StatusUnprocessableEntity), http.StatusUnprocessableEntity)
						return
					}
					if err := decoder.Decode(r.Body, mapped); err != nil {
						handle(ctx, fn, auth, w, fmt.Errorf("please provide valid '%v'", ctype))
						return
					}
				}
				//Scan in the path/query arguments.
				for _, param := range op.Parameters {
					if param.Location == parameterInVoid {
						continue
					}
					var (
						i          = param.Index[0]
						ref, deref reflect.Value
					)
					if argumentIsDirect := len(param.Index) == 1; argumentIsDirect {
						ref = args[i]

						if fn.In(i).Kind() != reflect.Ptr {
							ref = args[i].Addr()
							deref = args[i]
						} else {
							deref = args[i].Elem()
						}
					} else {
						//nested
						if fn.In(i).Kind() == reflect.Ptr {
							deref = fieldByIndex(args[i].Elem(), param.Index[1:])
						} else {
							deref = fieldByIndex(args[i], param.Index[1:])
						}
						ref = deref.Addr()
					}
					var items = 1
					var isSlice bool
					if deref.Kind() == reflect.Slice {
						isSlice = true
						if param.Location&parameterInQuery != 0 {
							items = len(r.URL.Query()[param.Name+"[]"])
							if items == 0 {
								items = len(r.URL.Query()[param.Name])
							}
							deref.Set(reflect.MakeSlice(deref.Type(), items, items))
						}
					}
					if param.Location == parameterInBody {
						if argumentsNeedsMapping {
							ref.Elem().Set(reflect.ValueOf(mapped).Elem().Field(mappedCount))
							mappedCount++
						} else {
							switch dst := ref.Interface().(type) {
							case *io.Reader:
								*dst = r.Body
							case *io.ReadCloser:
								*dst = r.Body
								closeBody = false
							case *fs.File:
								// Stream the request body as a file: the handler
								// reads bytes straight off the wire, with a
								// synthesized fs.FileInfo (name from the
								// Content-Disposition filename, size from
								// Content-Length). The handler owns closing it.
								*dst = newRequestFile(r)
								closeBody = false
							default:
								if !decoderOk {
									http.Error(w, http.StatusText(http.StatusUnprocessableEntity), http.StatusUnprocessableEntity)
									return
								}
								if err := decoder.Decode(r.Body, dst); err != nil {
									handle(ctx, fn, auth, w, fmt.Errorf("please provide a %v encoded %v (%w)", "json", args[i].Type().String(), err))
									return
								}
							}
						}
					}
					var idx int
					for val := ""; idx < items; idx++ {
						deref := deref
						ref := ref
						if isSlice {
							ref = deref.Index(idx).Addr()
							deref = deref.Index(idx)
						}
						if param.Location&parameterInPath != 0 {
							val = r.PathValue(param.Name)
						}
						if param.Location&parameterInQuery != 0 {
							if isSlice {
								vals := r.URL.Query()[param.Name+"[]"]
								if len(vals) == 0 {
									vals = r.URL.Query()[param.Name]
								}
								if idx < len(vals) {
									val = vals[idx]
								}
							} else {
								if v := r.URL.Query().Get(param.Name); v != "" {
									val = v
								}
							}
						}
						if !(param.Location == parameterInBody) {
							if val == "" {
							} else {
								if deref.Kind() == reflect.String {
									deref.SetString(val)

								} else if text, ok := ref.Interface().(encoding.TextUnmarshaler); ok {
									if err := text.UnmarshalText([]byte(val)); err != nil {
										handle(ctx, fn, auth, w, fmt.Errorf("please provide a valid %v (%w)", ref.Type().String(), err))
										return
									}
								} else if decoder, ok := ref.Interface().(json.Unmarshaler); ok {
									if _, err := strconv.ParseFloat(val, 64); err == nil || val == "true" || val == "false" {
										if err := decoder.UnmarshalJSON([]byte(val)); err == nil {
											goto decoded
										}
									}
									if err := decoder.UnmarshalJSON([]byte(strconv.Quote(val))); err != nil {
										handle(ctx, fn, auth, w, fmt.Errorf("please provide a valid %v (%w)", ref.Type().String(), err))
										return
									}
								} else if ok, err := scanTypeOf(ref, val); ok {
									if err != nil {
										handle(ctx, fn, auth, w, fmt.Errorf("please provide a valid %v (%w)", ref.Type().String(), err))
										return
									}
								} else {
									_, err := fmt.Sscanf(val, "%v", ref.Interface())
									if err != nil && err != io.EOF {
										handle(ctx, fn, auth, w, fmt.Errorf("please provide a valid %v (%w)", ref.Type().String(), err))
										return
									}
								}
							}
						}
					decoded:
						if ref.IsValid() && ref.CanAddr() {
							if reader, ok := ref.Interface().(http_api.HeaderReader); ok {
								reader.ReadHeadersHTTP(r.Header)
							}
						}
					}
				}
				if auth != nil {
					if err := auth.Authorize(ctx, r, fn, args); err != nil {
						handle(ctx, fn, auth, w, err)
						return
					}
				}
				//TODO decode body.
				results, err := fn.Call(ctx, args)
				if err != nil {
					handle(ctx, fn, auth, w, err)
					return
				}
				// Custom HTTP Headers Support
				// TODO cache whether or not we need to do this loop?
				header := w.Header()
				for _, val := range results {
					if writer, ok := val.Interface().(http_api.HeaderWriter); ok {
						writer.WriteHeadersHTTP(header)
					}
					if status, ok := val.Interface().(http_api.WithStatus); ok {
						w.WriteHeader(status.StatusHTTP())
					}
				}
				if len(results) == 0 {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				if len(results) == 1 {
					result := results[0]
					if result.Kind() == reflect.Chan && result.Type().ChanDir() == reflect.RecvDir {
						closeBody = false
						handleStreamingResult(ctx, r, w, result, fn, auth)
						return
					}
					if isSeq, isSeq2 := isIteratorType(result.Type()); isSeq || isSeq2 {
						handleStreamingResult(ctx, r, w, result, fn, auth)
						return
					}
				}
				if len(results) == 1 && op.DefaultContentType != "" {
					switch v := results[0].Interface().(type) {
					case io.WriterTo:
						w.Header().Set("Content-Type", string(op.DefaultContentType))
						if _, err := v.WriteTo(w); err != nil {
							handle(ctx, fn, auth, w, err)
						}
						return
					case io.ReadCloser:
						w.Header().Set("Content-Type", string(op.DefaultContentType))
						if _, err := io.Copy(w, v); err != nil {
							handle(ctx, fn, auth, w, err)
						}
						v.Close()
						return
					case *io.LimitedReader:
						w.Header().Set("Content-Type", string(op.DefaultContentType))
						w.Header().Set("Content-Length", strconv.Itoa(int(v.N)))
						if _, err := io.Copy(w, v); err != nil {
							handle(ctx, fn, auth, w, err)
						}
						return
					case io.Reader:
						w.Header().Set("Content-Type", string(op.DefaultContentType))
						if _, err := io.Copy(w, v); err != nil {
							handle(ctx, fn, auth, w, err)
						}
						return
					}
				}
				accept := r.Header.Get("Accept")
				if accept == "" || accept == "*/*" {
					if len(results) == 1 {
						switch results[0].Type().Kind() {
						case reflect.Struct, reflect.Slice, reflect.Map, reflect.Array:
							accept = "application/json"
						default:
							accept = "text/plain"
						}
					} else {
						accept = "application/json"
					}
				}
				for ctype := range strings.SplitSeq(accept, ",") {
					ctype, _, _ = mime.ParseMediaType(ctype)
					encoder, ok := contentTypes[ctype]
					if !ok {
						continue
					}
					w.Header().Set("Content-Type", ctype)
					if responseNeedsMapping {
						mapping := make(map[string]any)
						for i, rule := range resultRules {
							mapping[rule] = results[i].Interface()
						}
						if err := encoder.Encode(w, mapping); err != nil {
							handle(ctx, fn, auth, w, err)
						}
						return
					}
					if err := encoder.Encode(w, results[0].Interface()); err != nil {
						handle(ctx, fn, auth, w, err)
					}
					return
				}
				var supported []string
				for k := range contentTypes {
					supported = append(supported, k)
				}
				sort.Strings(supported)
				w.Header().Set("Accept-Encoding", strings.Join(supported, ", "))
				http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
			})) {
				return
			}
		}
		if !hasOptions {
			if !yield("OPTIONS "+path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				addCORS(auth, w, r, api.Function{})
				w.WriteHeader(http.StatusNoContent)
				return
			})) {
				return
			}
		}
		if !hasGet {
			if !yield("GET "+path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				formHandler{res: resource}.ServeHTTP(w, r)
				return
			})) {
				return
			}
		}
	}
}

// exercisedStructure runs all examples (via cache) and returns a filtered
// copy of the structure containing only functions exercised by at least one
// example. If impl does not implement WithExamples, the original structure
// is returned unmodified.
func exercisedStructure(ctx context.Context, impl any, structure api.Structure, cachedExample func(api.WithExamples, context.Context, string) (api.Example, bool)) api.Structure {
	documented, ok := impl.(api.WithExamples)
	if !ok {
		return structure
	}
	categories, err := documented.Examples(ctx)
	if err != nil || len(categories) == 0 {
		return structure
	}
	exercised := make(map[string]bool)
	for _, names := range categories {
		for _, name := range names {
			eg, ok := cachedExample(documented, ctx, name)
			if !ok {
				continue
			}
			for _, step := range eg.Steps {
				if step.Call != nil {
					if tag := string(step.Call.Tags.Get("rest")); tag != "" {
						exercised[tag] = true
					}
				}
			}
		}
	}
	if len(exercised) == 0 {
		return structure
	}
	return filterStructure(structure, exercised)
}

func filterStructure(structure api.Structure, exercised map[string]bool) api.Structure {
	filtered := structure
	filtered.Functions = nil
	for _, fn := range structure.Functions {
		if tag := string(fn.Tags.Get("rest")); tag != "" && exercised[tag] {
			filtered.Functions = append(filtered.Functions, fn)
		}
	}
	if structure.Namespace != nil {
		filtered.Namespace = make(map[string]api.Structure, len(structure.Namespace))
		for name, ns := range structure.Namespace {
			child := filterStructure(ns, exercised)
			if len(child.Functions) > 0 || len(child.Namespace) > 0 {
				filtered.Namespace[name] = child
			}
		}
	}
	return filtered
}
