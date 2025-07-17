package rest

import (
	"bytes"
	"context"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"runtime.link/api"
	http_api "runtime.link/api/internal/http"
	"runtime.link/api/internal/rtags"
	"runtime.link/api/xray"
	"runtime.link/xyz"
)

// API implements the [api.Linker] interface.
var API api.Linker[string, *http.Client] = linker{}

// Header returns a new default HTTP client that injects the given header into
// each request.
func Header(name, value string) *http.Client {
	return &http.Client{
		Transport: addHeaderToRequest{
			name:  name,
			value: value,
		},
	}
}

type addHeaderToRequest struct {
	name  string
	value string
}

func (h addHeaderToRequest) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set(h.name, h.value)
	defer req.Header.Del(h.name)
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, xray.New(err)
	}
	return resp, nil
}

type linker struct{}

// Link implements the [api.Linker] interface.
func (linker) Link(structure api.Structure, host string, client *http.Client) error {
	spec, err := specificationOf(structure)
	if err != nil {
		return xray.New(err)
	}
	if err := link(client, spec, host); err != nil {
		return xray.New(err)
	}
	return nil
}

func (op operation) encodeQuery(name string, query url.Values, rvalue reflect.Value) {
	if rvalue.IsValid() && !rvalue.IsZero() {
		if rvalue.Type().Implements(reflect.TypeOf([0]encoding.TextMarshaler{}).Elem()) {
			b, _ := rvalue.Interface().(encoding.TextMarshaler).MarshalText()
			query.Add(name, string(b))
		} else if rvalue.Type().Implements(reflect.TypeOf((*json.Marshaler)(nil)).Elem()) {
			b, _ := rvalue.Interface().(json.Marshaler).MarshalJSON()
			val := string(b)
			if val[0] == '"' {
				val, _ = strconv.Unquote(val)
			}
			query.Add(name, val)
		} else {
			if rvalue.Kind() == reflect.Slice {
				for i := range rvalue.Len() {
					op.encodeQuery(name, query, rvalue.Index(i))
				}
				return
			}
			query.Add(name, fmt.Sprintf("%v", rvalue.Interface()))
		}
	}
}

type RequestWriter struct {
	io.Writer
	header http.Header
}

func (w RequestWriter) WriteHeader(status int) {}

func (w RequestWriter) Header() http.Header {
	return w.header
}

func (op operation) clientWrite(header http.Header, path string, args []reflect.Value, body io.Writer, _ bool) (endpoint, contentType string, err error) {
	var encoder func(http.ResponseWriter, any) error
	contentType = string(op.DefaultContentType)
	ctype, ok := contentTypes[contentType]
	if !ok {
		return "", "", fmt.Errorf("unsupported content type: %v", op.DefaultContentType)
	}

	encoder = ctype.Encode

	writer := RequestWriter{Writer: body, header: header}

	var mapping map[string]any
	if op.argumentsNeedsMapping {
		mapping = make(map[string]any)
	}
	//deref is needed to prevent fmt from formatting the pointer as an address.
	deref := func(index []int) reflect.Value {
		value := args[index[0]]
		for value.Kind() == reflect.Ptr {
			if value.IsNil() {
				return reflect.Value{}
			}
			value = value.Elem()
		}
		if len(index) > 1 {
			return fieldByIndex(value, index[1:])
		}
		return value
	}
	var (
		query = make(url.Values)
	)
	for key, val := range op.Constants {
		query.Add(key, val)
	}
	for _, param := range op.Parameters {
		if param.Location == parameterInVoid {
			continue
		}
		if param.Location&parameterInPath != 0 {
			var value = fmt.Sprintf("%v", deref(param.Index).Interface())
			if !strings.HasSuffix(param.Name, "*") {
				value = url.PathEscape(value)
			}
			path = strings.Replace(path, "{"+param.Name+"}", value, 1)
		}
		if param.Location&parameterInQuery != 0 {
			op.encodeQuery(param.Name, query, deref(param.Index))
		}
		if param.Location == parameterInBody {
			if op.argumentsNeedsMapping {
				mapping[param.Name] = deref(param.Index).Interface()
			} else {
				if err := encoder(writer, deref(param.Index).Interface()); err != nil {
					return "", "", err
				}
			}
		}
	}
	if op.argumentsNeedsMapping {
		if err := encoder(writer, mapping); err != nil {
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
func (op operation) clientRead(mime string, results []reflect.Value, response io.ReadCloser) (close bool, err error) {
	if len(results) == 0 {
		return true, nil
	}
	// we want to support IO types
	if op.Tags.Get("mime") != "" {
		switch v := toPtr(&results[0]).(type) {
		case *io.WriterTo:
			*v = copier{response}
		case *io.ReadCloser:
			*v = response
			return false, nil
		case *io.Reader:
			*v = response
		case *[]byte:
			*v, err = io.ReadAll(response)
			if err != nil {
				return true, xray.New(err)
			}
		default:
			return true, fmt.Errorf("%v: 'mime' tag is not compatible with result value of type %T",
				strings.Join(append(op.Path, op.Name), "."),
				v,
			)
		}
		return true, nil
	}
	var (
		decoder func(io.Reader, any) error
	)
	ctype, ok := contentTypes[mime]
	if !ok {
		return true, fmt.Errorf("unsupported content type: %v", mime)
	}
	decoder = ctype.Decode
	//If there are custom response mapping rules,
	//then we decode into a map here.
	if op.responsesNeedsMapping {
		mapped := reflect.New(op.respMappingType).Interface()
		if err := decoder(response, mapped); err != nil {
			return true, xray.New(err)
		}
		for i := range op.respMappingType.NumField() {
			toPtr(&results[i])
			results[i].Set(reflect.ValueOf(mapped).Elem().Field(i))
		}
	} else {
		for i := range results {
			if err := decoder(response, toPtr(&results[i])); err != nil {
				return true, xray.New(err)
			}
		}
	}
	return true, nil
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
				headers := make(http.Header)
				endpoint, contentType, err := op.clientWrite(headers, path, args, writer, false)
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
				req.Header = headers
				xray.ContextAdd(ctx, req)

				//We are expecting JSON.
				req.Header.Add("Accept", "application/json")
				req.Header.Add("Content-Type", contentType)
				if debug {
					fmt.Println("headers:\n", req.Header)
				}
				resp, err := client.Do(req)
				if err != nil {
					return nil, err

				}
				var shouldClose = true
				defer func() {
					if shouldClose {
						resp.Body.Close()
					}
				}()
				resp.Body = xray.NewReader(ctx, resp.Body)
				xray.ContextAdd(ctx, resp)
				xray.ContextAdd(ctx, xyz.NewPair(req, resp))

				//Debug the reponse.
				if debug {
					fmt.Println("response:")
					b, _ := httputil.DumpResponse(resp, true)
					fmt.Println(string(b))
				}
				if resp.StatusCode < 200 || resp.StatusCode > 299 {
					return nil, decodeError(req, resp, spec)
				}
				//Zero out the results.
				for i := 0; i < fn.NumOut(); i++ {
					results[i] = reflect.Zero(fn.Type.Out(i))
				}
				ctype, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
				if ctype == "" {
					ctype = string(op.DefaultContentType)
				}
				if ctype == "" {
					ctype = "application/json"
				}
				if shouldClose, err = op.clientRead(ctype, results, resp.Body); err != nil {
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

var errType = reflect.TypeOf([0]error{}).Elem()

func decodeError(req *http.Request, resp *http.Response, spec specification) error {
	errortypes := spec.Instances[errType]
	if len(errortypes) == 1 && errortypes[0].Implements(errType) {
		err := reflect.New(errortypes[0])
		if json.NewDecoder(resp.Body).Decode(err.Interface()) == nil {
			return err.Elem().Interface().(error)
		}
	}
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
