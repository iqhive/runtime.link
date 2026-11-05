package rest

import (
	"bytes"
	"context"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/iqhive/runtime.link/api"
	http_api "github.com/iqhive/runtime.link/api/internal/http"
	"github.com/iqhive/runtime.link/api/internal/rtags"
	"github.com/iqhive/runtime.link/api/xray"
	"github.com/iqhive/runtime.link/xyz"
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
	resp, err := http.DefaultTransport.RoundTrip(http_api.WithHeaders(req, map[string]string{h.name: h.value}))
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
	if !rvalue.IsValid() || rvalue.IsZero() {
		return
	}
	switch {
	case hasTextForm(rvalue.Type()):
		query.Add(name, wireText(rvalue))
	case rvalue.Kind() == reflect.Pointer:
		// What a pointer points to is sent even when zero, the pointer is
		// what says the value is there.
		elem := rvalue.Elem()
		if elem.Kind() == reflect.Slice || elem.Kind() == reflect.Pointer || hasTextForm(elem.Type()) {
			op.encodeQuery(name, query, elem)
			return
		}
		query.Add(name, wireText(elem))
	case rvalue.Kind() == reflect.Slice:
		for i := range rvalue.Len() {
			op.encodeQuery(name, query, rvalue.Index(i))
		}
	default:
		query.Add(name, wireText(rvalue))
	}
}

func hasTextForm(rtype reflect.Type) bool {
	return rtype.Implements(reflect.TypeFor[encoding.TextMarshaler]()) || rtype.Implements(reflect.TypeFor[json.Marshaler]())
}

// wireText is the text of a value in a path or query, for the server to parse
// back: what %v prints for a type with a text form, a time.Time for one, is not
// that form.
func wireText(rvalue reflect.Value) string {
	switch value := rvalue.Interface().(type) {
	case encoding.TextMarshaler:
		b, _ := value.MarshalText()
		return string(b)
	case json.Marshaler:
		b, _ := value.MarshalJSON()
		val := string(b)
		if strings.HasPrefix(val, `"`) {
			val, _ = strconv.Unquote(val)
		}
		return val
	}
	return fmt.Sprintf("%v", rvalue.Interface())
}

// pathText is the text of a value in a path, escaped. A remainder keeps its
// slashes, and only those: anything else in it could end the path.
func pathText(rvalue reflect.Value, isRemainder bool) string {
	for rvalue.Kind() == reflect.Pointer && !rvalue.IsNil() {
		rvalue = rvalue.Elem()
	}
	var text string
	switch {
	case isNilResult(rvalue):
		text = ""
	case hasTextForm(rvalue.Type()):
		text = wireText(rvalue)
	case isByteSlice(rvalue.Type()):
		text = string(rvalue.Bytes())
	case rvalue.Kind() == reflect.Slice:
		parts := make([]string, rvalue.Len())
		for i := range parts {
			parts[i] = pathText(rvalue.Index(i), false)
		}
		return strings.Join(parts, ",")
	default:
		text = wireText(rvalue)
	}
	if !isRemainder {
		return url.PathEscape(text)
	}
	segments := strings.Split(text, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
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
	if ok {
		encoder = ctype.Encode
	} else {
		// The content type only matters once there is a body to encode: a
		// GET(text/html) endpoint, say, is still a plain path and query.
		encoder = func(http.ResponseWriter, any) error {
			return fmt.Errorf("unsupported content type: %v", op.DefaultContentType)
		}
	}

	writer := RequestWriter{Writer: body, header: header}

	var mapping map[string]any
	if op.argumentsNeedsMapping {
		mapping = make(map[string]any)
	}
	var elements []any
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
		// deref returns an invalid Value when the argument shape does not match
		// the parsed parameter index (e.g. a scalar where a struct field was
		// expected). Skip such parameters rather than dereferencing an invalid
		// Value, which would panic.
		value := deref(param.Index)
		if !value.IsValid() {
			if param.Location == parameterInBody && op.bodyArrayLength > 0 {
				elements = append(elements, nil)
			}
			continue
		}
		if param.expands {
			if value.Kind() == reflect.Map {
				for entries := value.MapRange(); entries.Next(); {
					op.encodeQuery(wireText(entries.Key()), query, entries.Value())
				}
			}
			continue
		}
		if param.Location&parameterInPath != 0 {
			str := pathText(value, strings.HasSuffix(param.Name, "*"))
			path = strings.Replace(path, "{"+param.Name+"}", str, 1)
		}
		if param.Location&parameterInQuery != 0 {
			op.encodeQuery(param.Name, query, value)
		}
		if param.Location == parameterInBody {
			switch {
			case param.optional:
				// What the body would hold is in the path or query already,
				// and the method has no body to send it in.
			case op.argumentsNeedsMapping:
				mapping[param.Name] = value.Interface()
			case op.bodyArrayLength > 0:
				elements = append(elements, value.Interface())
			default:
				// An fs.File / io.Reader body streams its raw bytes as the
				// request body (e.g. a file upload) rather than being encoded
				// as JSON. Content-Type becomes application/octet-stream so the
				// server binds the body directly. A filename from fs.File's
				// Stat is surfaced via Content-Disposition.
				if rdr, ok := value.Interface().(io.Reader); ok {
					if file, ok := value.Interface().(fs.File); ok {
						if info, err := file.Stat(); err == nil && info.Name() != "" {
							header.Set("Content-Disposition",
								fmt.Sprintf("attachment; filename=%q", info.Name()))
						}
					}
					if _, err := io.Copy(writer, rdr); err != nil {
						return "", "", err
					}
					contentType = "application/octet-stream"
					continue
				}
				if err := encoder(writer, value.Interface()); err != nil {
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
	if op.bodyArrayLength > 0 {
		contentType = "application/json"
		if err := contentTypes[contentType].Encode(writer, elements); err != nil {
			return "", "", err
		}
	}
	if len(query) == 0 {
		return path, contentType, nil
	}
	return path + "?" + query.Encode(), contentType, nil
}

type copier struct {
	from io.ReadCloser
}

func (c copier) WriteTo(w io.Writer) (n int64, err error) {
	defer c.from.Close()
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
			return false, nil
		case *io.ReadCloser:
			*v = response
			return false, nil
		case *io.Reader:
			// Reading to the end releases the connection, as it is the
			// caller's to read and this type has no Close to call.
			*v = response
			return false, nil
		case *[]byte:
			*v, err = io.ReadAll(response)
			if err != nil {
				return true, xray.New(err)
			}
		default:
			return true, fmt.Errorf("%v: 'mime' tag is not compatible with result value of type %T",
				op, v,
			)
		}
		return true, nil
	}
	var (
		decoder func(io.Reader, any) error
	)
	ctype, ok := contentTypes[mime]
	if !ok || ctype.Decode == nil {
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
			results[i] = reflect.ValueOf(mapped).Elem().Field(i)
		}
	} else {
		for i := range results {
			if isIterator(results[i].Type()) {
				seq, err := decodeIterator(decoder, response, results[i].Type())
				if err != nil {
					return true, xray.New(err)
				}
				results[i] = seq
				continue
			}
			// Through a pointer to the result, so that a result of type *T
			// can come back as nil.
			result := reflect.New(results[i].Type())
			if err := decoder(response, result.Interface()); err != nil {
				return true, xray.New(err)
			}
			results[i] = result.Elem()
		}
	}
	return true, nil
}

func isIterator(rtype reflect.Type) bool {
	isSeq, isSeq2 := isIteratorType(rtype)
	return isSeq || isSeq2
}

// decodeIterator reads what the server made of an iterator, a list, of maps for
// an iter.Seq2, and returns an iterator over it. The Go client asks for JSON, so
// this is not a stream.
func decodeIterator(decoder func(io.Reader, any) error, response io.Reader, rtype reflect.Type) (reflect.Value, error) {
	yieldType := rtype.In(0)
	if yieldType.NumIn() == 1 {
		items := reflect.New(reflect.SliceOf(yieldType.In(0)))
		if err := decoder(response, items.Interface()); err != nil {
			return reflect.Value{}, err
		}
		return iterateOver(rtype, func(yield reflect.Value) {
			for i := range items.Elem().Len() {
				if !yield.Call([]reflect.Value{items.Elem().Index(i)})[0].Bool() {
					return
				}
			}
		}), nil
	}
	pairs := reflect.New(reflect.SliceOf(reflect.MapOf(yieldType.In(0), yieldType.In(1))))
	if err := decoder(response, pairs.Interface()); err != nil {
		return reflect.Value{}, err
	}
	return iterateOver(rtype, func(yield reflect.Value) {
		for i := range pairs.Elem().Len() {
			for entries := pairs.Elem().Index(i).MapRange(); entries.Next(); {
				if !yield.Call([]reflect.Value{entries.Key(), entries.Value()})[0].Bool() {
					return
				}
			}
		}
	}), nil
}

func iterateOver(rtype reflect.Type, each func(yield reflect.Value)) reflect.Value {
	return reflect.MakeFunc(rtype, func(args []reflect.Value) []reflect.Value {
		each(args[0])
		return nil
	})
}

// zeroResult is the result of a response with no content. An iterator gets an
// empty one, as ranging over a nil one panics.
func zeroResult(rtype reflect.Type) reflect.Value {
	if isIterator(rtype) {
		return iterateOver(rtype, func(reflect.Value) {})
	}
	return reflect.Zero(rtype)
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

				var hasChannel bool
				var sendChan, recvChan reflect.Value
				for i := 0; i < fn.NumOut(); i++ {
					results[i] = zeroResult(fn.Type.Out(i))
					if fn.Type.Out(i).Kind() == reflect.Chan {
						chanDir := fn.Type.Out(i).ChanDir()
						if chanDir == reflect.BothDir {
							return nil, fmt.Errorf("bidirectional channels are not supported for WebSocket communication - use either chan<- (send) or <-chan (receive) for %v", fn.Type.Out(i))
						}
						hasChannel = true
						// A directional channel cannot be made, and the end
						// the caller does not hold is the library's to use.
						channel := reflect.MakeChan(reflect.ChanOf(reflect.BothDir, fn.Type.Out(i).Elem()), 0)
						results[i] = channel.Convert(fn.Type.Out(i))
						if chanDir == reflect.SendDir {
							sendChan = channel
						}
						if chanDir == reflect.RecvDir {
							recvChan = channel
						}
					}
				}

				if hasChannel {
					//body buffers what we will be sending to the endpoint.
					var writer = new(bytes.Buffer)
					//Figure out the REST endpoint to send a request to.
					//args are interpolated into the path and query as
					//defined in the "rest" tag for this function.
					headers := make(http.Header)
					endpoint, _, err := op.clientWrite(headers, path, args, writer, false)
					if err != nil {
						return nil, err
					}

					req, err := http.NewRequestWithContext(ctx, "GET", host+endpoint, nil)
					if err != nil {
						return nil, err
					}
					maps.Copy(req.Header, headers)

					req.Header.Add("Accept", "text/event-stream, application/json")
					stream, err := websocketOpen(ctx, client, req, sendChan, recvChan, func(resp *http.Response) error {
						return decodeError(req, resp, spec)
					})
					if err != nil {
						return nil, err
					}
					// A channel result needs something feeding it after this
					// call returns. It ends with the stream or with ctx, the
					// one the caller holds to end it.
					go stream()
					return results, nil
				}

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
				case "GET", "HEAD", "OPTIONS", "TRACE":
					body = http.NoBody
				case "DELETE":
					// Unless it has an argument for one.
					body = http.NoBody
					if writer.Len() > 0 {
						body = io.NopCloser(writer)
					}
				default:
					body = io.NopCloser(writer)
				}
				httpMethod := method
				if method == "QUERY" {
					httpMethod = "POST"
				}
				req, err := http.NewRequestWithContext(ctx, httpMethod, host+endpoint, xray.NewReader(ctx, body))
				if err != nil {
					return nil, err
				}
				if method != httpMethod {
					req.Header.Set("X-HTTP-Method-Override", method)
				}
				maps.Copy(req.Header, headers)
				xray.ContextAdd(ctx, req)

				//We are expecting JSON.
				req.Header.Add("Accept", "application/json")
				if req.Header.Get("Content-Type") == "" {
					req.Header.Set("Content-Type", contentType)
				}
				if debug {
					fmt.Println("headers:\n", req.Header)
				}
				resp, err := send(ctx, client, req)
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
				if resp.StatusCode == http.StatusNoContent {
					return results, nil
				}
				ctype, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
				if ctype == "" {
					ctype = string(op.DefaultContentType)
				}
				if ctype == "" || ctype == "text/json" {
					ctype = "application/json"
				}
				if shouldClose, err = op.clientRead(ctype, results, resp.Body); err != nil {
					return nil, err
				}
				// Custom Headers support.
				// TODO cache whether or not we need to do this loop?
				for i := range results {
					// A copy that can be addressed, as the method of a header
					// reader is on the pointer receiver and it has to keep
					// what was decoded.
					addressable := reflect.New(results[i].Type()).Elem()
					addressable.Set(results[i])
					if reader, ok := headerReaderOf(addressable); ok {
						reader.ReadHeadersHTTP(resp.Header)
						results[i] = addressable
					}
				}
				return results, nil
			})
		}
	}
	return nil
}

var errType = reflect.TypeOf([0]error{}).Elem()

// decodesAsError reports whether body starts with the error v points to, which it
// then holds, whatever follows, as such a body always has been. A body of text
// that starts with the word null is a message, not an error with nothing in it.
func decodesAsError(body []byte, v any) bool {
	if json.Unmarshal(body, v) == nil {
		return true
	}
	if bytes.HasPrefix(bytes.TrimSpace(body), []byte("null")) {
		return false
	}
	return json.NewDecoder(bytes.NewReader(body)).Decode(v) == nil
}

func decodeError(req *http.Request, resp *http.Response, spec specification) error {
	// Read in advance, as a body that turns out not to be this error is still
	// the message for the error it is. Bounded, this comes from the network.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxDecodedBody))
	errortypes := spec.Instances[errType]
	if len(errortypes) == 1 && errortypes[0].Implements(errType) {
		err := reflect.New(errortypes[0])
		if decodesAsError(body, err.Interface()) {
			if reader, ok := err.Interface().(http_api.HeaderReader); ok {
				reader.ReadHeadersHTTP(resp.Header)
			}
			return err.Elem().Interface().(error)
		}
	}
	read := *resp
	read.Body = io.NopCloser(bytes.NewReader(body))
	resp = &read
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
