package rest

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
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
	var router = http.NewServeMux()
	spec, err := specificationOf(api.StructureOf(impl))
	if err != nil {
		return xray.Error(err)
	}
	attach(auth, router, spec)
	return http.ListenAndServe(addr, router)
}

// Handler returns a HTTP handler that serves supported API types.
func Handler(auth api.Auth[*http.Request], impl any) (http.Handler, error) {
	var router = http.NewServeMux()
	spec, err := specificationOf(api.StructureOf(impl))
	if err != nil {
		return nil, xray.Error(err)
	}
	attach(auth, router, spec)
	return router, nil
}

func attach(auth api.Auth[*http.Request], router *http.ServeMux, spec specification) {
	for path, resource := range spec.Resources {
		for method, operation := range resource.Operations {
			var (
				op   = operation
				fn   = op.Function
				path = rtags.CleanupPattern(path)

				mimetype    = string(fn.Tags.Get("mime"))
				resultRules = rtags.ResultRulesOf(string(fn.Tags.Get("rest")))

				responseNeedsMapping  = len(resultRules) > 0
				argumentsNeedsMapping = len(rtags.ArgumentRulesOf(string(fn.Tags.Get("rest")))) > 0
			)
			if method == "GET" {
				router.HandleFunc("OPTIONS "+path, func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
				})
			}
			router.HandleFunc(string(method)+" "+path, func(w http.ResponseWriter, r *http.Request) {
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
					var (
						val = ""
					)
					if param.Location&parameterInPath != 0 {
						val = r.PathValue(param.Name)
					}
					if param.Location&parameterInQuery != 0 {
						if v := r.URL.Query().Get(param.Name); v != "" {
							val = v
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
							} else {
								_, err := fmt.Sscanf(val, "%v", ref.Interface())
								if err != nil && err != io.EOF {
									handle(w, fmt.Errorf("please provide a valid %v (%w)", ref.Type().String(), err))
									return
								}
							}
						}
					}
					if ref.IsValid() && ref.CanAddr() {
						if reader, ok := ref.Interface().(http_api.HeaderReader); ok {
							reader.ReadHeadersHTTP(r.Header)
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
				}
				var mapping map[string]interface{}
				if responseNeedsMapping {
					mapping = make(map[string]interface{})
					for i, rule := range resultRules {
						mapping[rule] = results[i].Interface()
					}
					b, err := json.Marshal(mapping)
					if err != nil {
						handle(w, err)
					}
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Content-Length", strconv.Itoa(len(b)))
					w.Write(b)
					return
				}
				// Endpoints can define a mime tag to overide the default JSON marshaling behaviour.
				// This is useful for serving files.
				if mimetype != "" {
					if len(results) != 1 {
						handle(w, fmt.Errorf("%v: the 'mime' tag is not supported for multiple return values",
							strings.Join(append(fn.Path, fn.Name), ".")))
						return
					}
					w.Header().Set("Content-Type", mimetype)
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
					case []byte:
						w.Header().Set("Content-Length", strconv.Itoa(len(v)))
						if _, err := w.Write(v); err != nil {
							handle(w, err)
						}
						return
					}
				}
				if len(results) == 1 {
					//It may be useful to be able to override the default json
					//marshalling behaviour of this package.
					if marshaler, ok := results[0].Interface().(marshaler); ok {
						b, err := marshaler.MarshalREST()
						if err != nil {
							handle(w, err)
						}
						if _, err := w.Write(b); err != nil {
							handle(w, err)
						}
						return
					}
					b, err := json.Marshal(results[0].Interface())
					if err != nil {
						handle(w, err)
					}
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Content-Length", strconv.Itoa(len(b)))
					w.Write(b)
					return
				}
				var (
					converted = make([]interface{}, 0, len(results))
				)
				for _, v := range results {
					converted = append(converted, v.Interface())
				}
				b, err := json.Marshal(converted)
				if err != nil {
					handle(w, err)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Length", strconv.Itoa(len(b)))
				w.Write(b)
			})
		}
	}
}
