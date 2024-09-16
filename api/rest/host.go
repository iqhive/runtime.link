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
	"mime"
	"net/http"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"runtime.link/api"
	http_api "runtime.link/api/internal/http"
	"runtime.link/api/internal/oas"
	"runtime.link/api/internal/rtags"
	"runtime.link/api/xray"
)

// ListenAndServe starts a HTTP server that serves supported API
// types. If the [Authenticator] is nil, requests will not require
// any authentication.
func ListenAndServe(addr string, auth api.Auth[*http.Request], impl any) error {
	handler, err := Handler(auth, impl)
	if err != nil {
		return xray.New(err)
	}
	return http.ListenAndServe(addr, handler)
}

var (
	//go:embed docs_head.html
	docs_head []byte
	//go:embed docs_body.html
	docs_body []byte
)

// Handler returns a HTTP handler that serves supported API types.
func Handler(auth api.Auth[*http.Request], impl any) (http.Handler, error) {
	var router = new(mux)
	notfound := http.NotFoundHandler()
	router.for404 = &notfound
	spec, err := specificationOf(api.StructureOf(impl))
	if err != nil {
		return nil, xray.New(err)
	}

	docs, err := oasDocumentOf(spec.Structure)
	if err != nil {
		return nil, xray.New(err)
	}
	rtype := reflect.TypeOf(impl)
	docs.Information.Title = oas.Readable(path.Base(rtype.PkgPath()) + " " + rtype.Name())

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.Encode(docs)

	code, err := sdkFor(docs)
	if err != nil {
		return nil, xray.New(err)
	}

	router.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if auth != nil {
			if _, err := auth.Authenticate(r, api.Function{}); err != nil {
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
	})
	if documented, ok := impl.(api.WithExamples); ok {
		router.HandleFunc("GET /examples/{name}", func(w http.ResponseWriter, r *http.Request) {
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
			if err == nil && len(examples) > 0 {
				w.Write([]byte("<nav style='min-height: 100vh;'>"))
				fmt.Fprintf(w, "<h2><a href=\"../\">API Reference</a></h2>")
				w.Write([]byte("<h3>Examples:</h3>"))
				for _, example := range examples {
					fmt.Fprintf(w, "<a href=\"%v\">%[1]v</a>", example)
				}
				w.Write([]byte("</nav>"))
			}
			w.Write([]byte("<main>"))
			defer w.Write([]byte("</main></body></html>"))
			header := "#" + example.Title + " " + example.Story
			if example.Error == nil {
				header = "✅ " + header
			} else {
				header = "❌ " + header
			}
			fmt.Fprintf(w, "<h1>%v</h1>", example.Title)
			fmt.Fprintf(w, "<p>%v</p>", example.Story)
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
					fmt.Fprintf(w, "<div class=sample><pre>%v</pre>", url)
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
		})
	}
	attach(auth, router, spec)
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

func attach(auth api.Auth[*http.Request], router *mux, spec specification) {
	for path, resource := range spec.Resources {
		var hasGet = false
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
				router.HandleFunc("OPTIONS "+path, func(w http.ResponseWriter, r *http.Request) {
					if auth != nil {
						if _, err := auth.Authenticate(r, fn); err != nil {
							handle(r.Context(), fn, auth, w, err)
							return
						}
					}
					w.WriteHeader(200)
				})
			}
			if method == "GET" {
				hasGet = true
			}
			router.HandleFunc(string(method)+" "+path, func(w http.ResponseWriter, r *http.Request) {
				var (
					ctx = r.Context()
					err error
				)
				var closeBody bool = true
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
				decoder, ok := contentTypes[ctype]
				if !ok {
					http.Error(w, http.StatusText(http.StatusUnprocessableEntity), http.StatusUnprocessableEntity)
					return
				}
				var args = make([]reflect.Value, fn.NumIn())
				for i := range args {
					args[i] = reflect.New(fn.In(i)).Elem()
				}
				var mapped any
				var mappedCount int
				if argumentsNeedsMapping {
					mapped = reflect.New(op.argMappingType).Interface()
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
							deref = args[i].Elem().FieldByIndex(param.Index[1:])
						} else {
							deref = args[i].FieldByIndex(param.Index[1:])
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
					websocketServeHTTP(ctx, r, w, results[0])
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
			})
		}
		if !hasGet {
			router.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
				formHandler{res: resource}.ServeHTTP(w, r)
				return
			})
		}
	}
}
