package rest

import (
	"bytes"
	"context"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
	"strings"

	"runtime.link/api"
	http_api "runtime.link/api/internal/http"
	"runtime.link/api/internal/rtags"
	"runtime.link/api/xray"
	"runtime.link/xyz"
)

type bodyEncoder interface {
	Encode(any) error
}

func (op operation) clientWrite(path string, args []reflect.Value, body io.Writer, indent bool) (endpoint, contentType string, err error) {
	var encoder bodyEncoder
	switch op.DefaultContentType {
	case "application/json":
		encoder := json.NewEncoder(body)
		if indent {
			encoder.SetIndent("", "\t")
		}
	case "multipart/form-data":
		multipart := newMultipartEncoder(body)
		contentType = fmt.Sprintf("multipart/form-data; boundary=%v", multipart.w.Boundary())
		encoder = multipart
	default:
		return "", "", fmt.Errorf("unsupported content type: %v", op.DefaultContentType)
	}
	var mapping map[string]interface{}
	if op.argumentsNeedsMapping {
		mapping = make(map[string]interface{})
	}
	//deref is needed to prevent fmt from formatting the pointer as an address.
	deref := func(index []int) reflect.Value {
		value := args[index[0]]
		for value.Kind() == reflect.Ptr {
			value = value.Elem()
		}
		if len(index) > 1 {
			return value.FieldByIndex(index[1:])
		}
		return value
	}
	var (
		query = make(url.Values)
	)
	for _, param := range op.Parameters {
		if param.Location == parameterInVoid {
			continue
		}
		if param.Location&parameterInPath != 0 {
			path = strings.Replace(path, "{"+param.Name+"}", url.PathEscape(fmt.Sprintf("%v", deref(param.Index).Interface())), 1)
		}
		if param.Location&parameterInQuery != 0 {
			val := deref(param.Index)
			if val.IsValid() && !val.IsZero() {
				if val.Type().Implements(reflect.TypeOf([0]encoding.TextMarshaler{}).Elem()) {
					b, _ := val.Interface().(encoding.TextMarshaler).MarshalText()
					query.Add(param.Name, string(b))
				} else {
					query.Add(param.Name, fmt.Sprintf("%v", val.Interface()))
				}
			}
		}
		if param.Location == parameterInBody {
			if op.argumentsNeedsMapping {
				mapping[param.Name] = deref(param.Index).Interface()
			} else {
				if err := encoder.Encode(deref(param.Index).Interface()); err != nil {
					return "", "", err
				}
			}
		}
	}
	if op.argumentsNeedsMapping {
		if err := encoder.Encode(mapping); err != nil {
			return "", "", err
		}
		if debug {
			b, _ := json.MarshalIndent(mapping, "", "\t")
			fmt.Println(string(b))
		}
	}
	if len(query) == 0 {
		return path, contentType, nil
	}
	return path + "?" + query.Encode(), contentType, nil
}

type copier struct {
	from io.Reader
}

func (c copier) WriteTo(w io.Writer) (n int64, err error) {
	return io.Copy(w, c.from)
}

// results argument need to be preallocated.
func (op operation) clientRead(results []reflect.Value, response io.Reader, resultRules []string) (err error) {
	if len(results) == 0 {
		return nil
	}
	// we want to support IO types
	if op.Tags.Get("mime") != "" {
		switch v := toPtr(&results[0]).(type) {
		case *io.WriterTo:
			*v = copier{response}
		case *io.ReadCloser:
			*v = io.NopCloser(response)
		case *io.Reader:
			*v = response
		case *[]byte:
			*v, err = ioutil.ReadAll(response)
			if err != nil {
				return xray.Error(err)
			}
		default:
			return fmt.Errorf("%v: 'mime' tag is not compatible with result value of type %T",
				strings.Join(append(op.Path, op.Name), "."),
				v,
			)
		}
		return nil
	}
	var (
		responseNeedsMapping = len(resultRules) > 0
		decoder              = json.NewDecoder(response)
	)
	//If there are custom response mapping rules,
	//then we decode into a map here.
	var mapping map[string]json.RawMessage
	if responseNeedsMapping {
		mapping = make(map[string]json.RawMessage)
		if err := decoder.Decode(&mapping); err != nil {
			return xray.Error(err)
		}
		if debug {
			for key := range mapping {
				fmt.Println(key)
			}
		}
		for i, rule := range resultRules {
			if debug {
				fmt.Println("copying", rule, "into", i, results[i].Type())
			}
			//Write into a return value (as usual)
			if err := json.Unmarshal(mapping[rule], toPtr(&results[i])); err != nil {
				return xray.Error(err)
			}
		}
	} else {
		for i := range results {
			if err := decoder.Decode(toPtr(&results[i])); err != nil {
				return xray.Error(err)
			}
		}
	}
	return nil
}

func link(client *http.Client, spec specification, host string) error {
	if host == "" {
		host = spec.Host.Get("www")
	}
	if client == nil {
		client = http.DefaultClient
	}
	for path, resource := range spec.Resources {
		for method, operation := range resource.Operations {
			var (
				op = operation
				fn = op.Function
			)
			if debug {
				fmt.Println(path, fn.Name)
			}
			var (
				method = string(method) //Determine the HTTP method of this request.
				path   = rtags.CleanupPattern(path)

				resultRules = rtags.ResultRulesOf(string(fn.Tags.Get("rest")))
			)
			//Create an implementation of the function that calls the REST
			//endpoint over the network and returns the results.
			fn.Make(func(ctx context.Context, args []reflect.Value) (results []reflect.Value, err error) {
				if host == "" {
					return nil, fmt.Errorf("failed to call %v, %s host URL is empty", path, spec.Name)
				}
				results = make([]reflect.Value, fn.NumOut())
				//body buffers what we will be sending to the endpoint.
				var writer = new(bytes.Buffer)
				//Figure out the REST endpoint to send a request to.
				//args are interpolated into the path and query as
				//defined in the "rest" tag for this function.
				endpoint, contentType, err := op.clientWrite(path, args, writer, false)
				if err != nil {
					return nil, err
				}
				//Debug the url.
				if debug {
					fmt.Println(method, host+endpoint)
					fmt.Println("body:\n", writer.String())
				}
				var body io.ReadCloser
				// These methods should not have a body.
				switch method {
				case "GET", "HEAD", "DELETE", "OPTIONS", "TRACE":
					body = http.NoBody
				default:
					body = io.NopCloser(writer)
				}
				req, err := http.NewRequestWithContext(ctx, method, host+endpoint, xray.NewReader(ctx, body))
				if err != nil {
					return nil, err
				}
				xray.Add(ctx, req)

				//We are expecting JSON.
				req.Header.Set("Accept", "application/json")
				req.Header.Set("Content-Type", contentType)
				if debug {
					fmt.Println("headers:\n", req.Header)
				}
				resp, err := client.Do(req)
				if err != nil {
					return nil, err

				}
				defer resp.Body.Close()
				resp.Body = xray.NewReader(ctx, resp.Body)
				xray.Add(ctx, resp)
				xray.Add(ctx, xyz.NewPair(req, resp))

				//Debug the reponse.
				if debug {
					fmt.Println("response:")
					b, _ := httputil.DumpResponse(resp, true)
					fmt.Println(string(b))
				}
				if resp.StatusCode < 200 || resp.StatusCode > 299 {
					return nil, decodeError(req, resp, spec, fn, err)
				}
				//Zero out the results.
				for i := 0; i < fn.NumOut(); i++ {
					results[i] = reflect.Zero(fn.Type.Out(i))
				}
				if err := op.clientRead(results, resp.Body, resultRules); err != nil {
					return nil, err
				}
				// Custom Headers support.
				// TODO cache whether or not we need to do this loop?
				for i := range results {
					if results[i].Type().Implements(reflect.TypeOf((*http_api.HeaderReader)(nil)).Elem()) {
						if writer, ok := toPtr(&results[i]).(http_api.HeaderReader); ok {
							writer.ReadHeadersHTTP(resp.Header)
						}
					}
				}
				return results, nil
			})
		}
	}
	return nil
}

func decodeError(req *http.Request, resp *http.Response, spec specification, fn api.Function, err error) error {
	var wrap func(error) error = func(err error) error { return err } // we choose which api error to wrap with.
	if resp.StatusCode == 404 {
		if req.Method == "DELETE" {
			return nil
		}
		return http_api.ErrNotFound
	}
	if err := http_api.ResponseError(resp); err != nil {
		return wrap(err)
	}
	return wrap(errors.New("unexpected status : " + resp.Status))
}
