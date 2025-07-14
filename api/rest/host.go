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
	"iter"
	"mime"
	"net/http"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"runtime.link/api"
	"runtime.link/api/cors"
	http_api "runtime.link/api/internal/http"
	"runtime.link/api/internal/oas"
	"runtime.link/api/internal/rtags"
	"runtime.link/api/xray"
)

func apiReferenceURL(fn api.Function) string {
	var categoryName string
	if len(fn.Path) == 0 {
		categoryName = "default"
	} else {
		categoryName = fn.Path[len(fn.Path)-1]
	}
	
	return fmt.Sprintf("../#/%s/%s", categoryName, fn.Name)
}

var (
	//go:embed docs_head.html
	docs_head []byte
	//go:embed docs_body.html
	docs_body []byte
)

func fieldByIndex(value reflect.Value, index []int) reflect.Value {
	if len(index) == 1 {
		return value.Field(index[0])
	}
	for i, x := range index {
		if i > 0 {
			if value.Kind() == reflect.Pointer && value.Type().Elem().Kind() == reflect.Struct {
				if value.IsNil() {
					value.Set(reflect.New(value.Type().Elem()))
				}
				value = value.Elem()
			}
		}
		value = value.Field(x)
	}
	return value
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
	docs, err := oasDocumentOf(spec.Structure)
	if err != nil {
		return nil, xray.New(err)
	}
	if docs.Information.Title == "" {
		rtype := reflect.TypeOf(impl)
		docs.Information.Title = oas.Readable(path.Base(rtype.PkgPath()) + " " + rtype.Name())
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.Encode(docs)
	code, err := sdkFor(docs)
	if err != nil {
		return nil, xray.New(err)
	}
	return func(yield func(string, http.Handler) bool) {
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
		if !yield("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auth != nil {
				addCORS(auth, w, r, api.Function{})
				if _, err := auth.Authenticate(r, api.Function{}); err != nil {
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
				w.Write(buf.Bytes())
				return
			}
			if strings.Contains(r.Header.Get("Accept"), "application/javascript") {
				w.Header().Set("Content-Type", "application/javascript")
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
		})) {
			return
		}
		if documented, ok := impl.(api.WithExamples); ok {
			if !yield("GET /examples/{name}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				addCORS(auth, w, r, api.Function{})
				name := r.PathValue("name")
				example, ok := documented.Example(r.Context(), name)
				if !ok {
					http.NotFound(w, r)
					return
				}
				w.Write([]byte("<!DOCTYPE html>"))
				w.Write(docs_head)
				w.Write([]byte("<body>"))
				examples, err := documented.Examples(r.Context())
				if err == nil {
					w.Write([]byte("<nav>"))
					fmt.Fprintf(w, "<h2><a href=\"../\">← API Reference</a></h2>")
					w.Write([]byte("<h3>Examples</h3>"))
					
					w.Write([]byte("<div class=\"examples-list\">"))
					for category, categoryExamples := range examples {
						isCurrentCategory := false
						for _, exampleName := range categoryExamples {
							if exampleName == name {
								isCurrentCategory = true
								break
							}
						}
						
						if isCurrentCategory {
							fmt.Fprintf(w, "<details class=\"example-category\" open>")
						} else {
							fmt.Fprintf(w, "<details class=\"example-category\">")
						}
						
						fmt.Fprintf(w, "<summary class=\"category-header\">%s</summary>", strings.Title(category))
						fmt.Fprintf(w, "<div class=\"category-examples\">")
						for _, exampleName := range categoryExamples {
							title := formatPascalCaseTitle(exampleName)
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
				fmt.Fprintf(w, "<h1>%v</h1>", html.EscapeString(example.Title))
				if example.Story != "" {
					fmt.Fprintf(w, "<p>%v</p>", html.EscapeString(example.Story))
				}
				if example.Tests != "" {
					fmt.Fprintf(w, "<p>Tests %s</p>", html.EscapeString(example.Tests))
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
						fmt.Fprintf(w, "<p>%s</p>", step.Note)
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
						apiRefURL := apiReferenceURL(*step.Call)
						fmt.Fprintf(w, "<div class=sample><pre>%v <a href=\"%s\" target=\"_blank\" class=\"api-ref-link\">📖 View in API Reference</a></pre>", url, apiRefURL)
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
					ctx, err = auth.Authenticate(r, fn)
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
					if deref.Kind() == reflect.Slice {
						if param.Location&parameterInQuery != 0 {
							items = len(r.URL.Query()[param.Name+"[]"])
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
						if items > 1 {
							ref = deref.Index(idx).Addr()
							deref = deref.Index(idx)
						}
						if param.Location&parameterInPath != 0 {
							val = r.PathValue(param.Name)
						}
						if param.Location&parameterInQuery != 0 {
							if items > 1 {
								vals := r.URL.Query()[param.Name+"[]"]
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
				if len(results) == 1 && results[0].Kind() == reflect.Chan && results[0].Type().ChanDir() == reflect.RecvDir {
					closeBody = false
					websocketServeHTTP(ctx, r, w, results[0], reflect.Value{})
					return
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
				ctypes := strings.Split(accept, ",")
				for _, ctype := range ctypes {
					ctype, _, _ = mime.ParseMediaType(ctype)
					encoder, ok := contentTypes[ctype]
					if !ok {
						continue
					}
					w.Header().Set("Content-Type", ctype)
					if responseNeedsMapping {
						mapping := make(map[string]interface{})
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
