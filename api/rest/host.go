package rest

import (
	"bytes"
	_ "embed"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"runtime.link/api"
	http_api "runtime.link/api/internal/http"
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

//go:embed docs.html
var html []byte

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
	docs.Information.Title = "Pet Store"

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.Encode(docs)

	code, err := sdkFor(docs)
	if err != nil {
		return nil, xray.New(err)
	}

	router.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
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
			w.Write(html)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		return
	})

	attach(auth, router, spec)
	return router, nil
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
					w.WriteHeader(200)
				})
			}
			if method == "GET" {
				hasGet = true
			}
			router.HandleFunc(string(method)+" "+path, func(w http.ResponseWriter, r *http.Request) {
				if method == "GET" && strings.Contains(r.Header.Get("Accept"), "text/html") || strings.Contains(r.Header.Get("Accept"), "application/schema+json") {
					formHandler{res: resource}.ServeHTTP(w, r)
					return
				}
				var (
					ctx = r.Context()
					err error
				)
				handle := func(rw http.ResponseWriter, err error) {
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
					for _, scenario := range op.Function.Root.Scenarios {
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
				var closeBody bool = true
				defer func() {
					if closeBody {
						r.Body.Close()
					}
				}()
				if auth != nil {
					ctx, err = auth.Authenticate(r, fn)
					if err != nil {
						handle(w, err)
						return
					}
				}
				var args = make([]reflect.Value, fn.NumIn())
				for i := range args {
					args[i] = reflect.New(fn.In(i)).Elem()
				}
				var argMapping map[string]json.RawMessage
				if argumentsNeedsMapping {
					argMapping = make(map[string]json.RawMessage)
					if err := json.NewDecoder(r.Body).Decode(&argMapping); err != nil {
						handle(w, err)
						return
					}
					for i, param := range op.Parameters {
						if param.Location == parameterInBody {
							raw := argMapping[param.Name]
							if raw == nil {
								continue
							}
							if err := json.Unmarshal(raw, toPtr(&args[i])); err != nil {
								handle(w, err)
								return
							}
						}
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
					if param.Location == parameterInBody && !argumentsNeedsMapping {
						switch dst := ref.Interface().(type) {
						case *io.Reader:
							*dst = r.Body
						case *io.ReadCloser:
							*dst = r.Body
							closeBody = false
						default:
							if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
								handle(w, fmt.Errorf("please provide a %v encoded %v (%w)", "json", args[i].Type().String(), err))
								return
							}
						}
					}
					var items = 1
					if deref.Kind() == reflect.Slice {
						if param.Location&parameterInQuery != 0 {
							items = len(r.URL.Query()[param.Name+"[]"])
						}
						deref.Set(reflect.MakeSlice(deref.Type(), items, items))
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
										handle(w, fmt.Errorf("please provide a valid %v (%w)", ref.Type().String(), err))
										return
									}
								} else if decoder, ok := ref.Interface().(json.Unmarshaler); ok {
									if _, err := strconv.ParseFloat(val, 64); err == nil || val == "true" || val == "false" {
										if err := decoder.UnmarshalJSON([]byte(val)); err == nil {
											goto decoded
										}
									}
									if err := decoder.UnmarshalJSON([]byte(strconv.Quote(val))); err != nil {
										handle(w, fmt.Errorf("please provide a valid %v (%w)", ref.Type().String(), err))
										return
									}
								} else {
									_, err := fmt.Sscanf(val, "%v", ref.Interface())
									if err != nil && err != io.EOF {
										handle(w, fmt.Errorf("please provide a valid %v (%w)", ref.Type().String(), err))
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
					if err := auth.Authorize(r, fn, nil); err != nil {
						handle(w, err)
						return
					}
				}
				//TODO decode body.
				results, err := fn.Call(ctx, args)
				if err != nil {
					handle(w, err)
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
					switch v := results[0].Interface().(type) {
					case io.WriterTo:
						if _, err := v.WriteTo(w); err != nil {
							handle(w, err)
						}
						return
					case io.ReadCloser:
						if _, err := io.Copy(w, v); err != nil {
							handle(w, err)
						}
						v.Close()
						return
					case *io.LimitedReader:
						w.Header().Set("Content-Length", strconv.Itoa(int(v.N)))
						if _, err := io.Copy(w, v); err != nil {
							handle(w, err)
						}
						return
					case io.Reader:
						if _, err := io.Copy(w, v); err != nil {
							handle(w, err)
						}
						return
					}
				}
				accept := r.Header.Get("Accept")
				if accept == "" {
					if len(results) == 1 {
						switch results[0].Type().Kind() {
						case reflect.Struct, reflect.Slice, reflect.Map, reflect.Array:
							accept = "application/json"
						default:
							accept = "text/plain"
						}
					}
				}
				ctypes := strings.Split(accept, ",")
				for _, ctype := range ctypes {
					encoder, ok := builtinEncoders[ctype]
					if !ok {
						continue
					}
					w.Header().Set("Content-Type", ctype)
					if responseNeedsMapping {
						mapping := make(map[string]interface{})
						for i, rule := range resultRules {
							mapping[rule] = results[i].Interface()
						}
						if err := encoder(w, mapping); err != nil {
							handle(w, err)
						}
						return
					}
					if err := encoder(w, results[0].Interface()); err != nil {
						handle(w, err)
					}
					return
				}
				var supported []string
				for k := range builtinEncoders {
					supported = append(supported, k)
				}
				sort.Strings(supported)
				w.Header().Set("Accept-Encoding", strings.Join(supported, ","))
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
