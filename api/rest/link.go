package rest

import (
	"bytes"
	"context"
	"encoding"
	"encoding/json"
	"encoding/xml"
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
	"time"

	"github.com/iqhive/runtime.link/api"
	http_api "github.com/iqhive/runtime.link/api/internal/http"
	"github.com/iqhive/runtime.link/api/internal/rtags"
	"github.com/iqhive/runtime.link/api/xray"
	"github.com/iqhive/runtime.link/xyz"
)

// API implements the [api.Linker] interface. Every function with a rest tag is
// linked: of two with one method and path, each calls that route.
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

func (op operation) encodeQuery(name string, query url.Values, rvalue reflect.Value) error {
	return op.encodeQueryValue(name, query, rvalue, false)
}

// A pointer, map entry or slice element says a value is present even when zero.
// Standalone zero scalars retain their existing omission behavior.
func (op operation) encodeQueryValue(name string, query url.Values, rvalue reflect.Value, present bool) error {
	if isNilResult(rvalue) || (!present && rvalue.IsZero()) {
		return nil
	}
	if _, ok := pointedTo(rvalue.Type()); !ok {
		return fmt.Errorf("%s: cannot write the query parameter %q: %w", op.Name, name, errEndlessPointer)
	}
	switch {
	case isSentWhole(rvalue.Type()):
	case rvalue.Kind() == reflect.Pointer:
		return op.encodeQueryValue(name, query, rvalue.Elem(), true)
	case rvalue.Kind() == reflect.Slice:
		for i := range rvalue.Len() {
			element := rvalue.Index(i)
			if isNilResult(element) {
				// A nil keeps its place in the list, as the server reads an
				// empty element as one: left out, those after it would move up.
				query.Add(name, "")
				continue
			}
			if err := op.encodeQueryValue(name, query, element, true); err != nil {
				return err
			}
		}
		return nil
	}
	text, err := wireText(rvalue)
	// Added even when it could not be written, and is empty: a sample shows
	// the call that was made, and a request is not sent.
	query.Add(name, text)
	if err != nil {
		return fmt.Errorf("%s: cannot write the query parameter %q: %w", op.Name, name, err)
	}
	return nil
}

func hasTextForm(rtype reflect.Type) bool {
	return rtype.Implements(reflect.TypeFor[encoding.TextMarshaler]()) || rtype.Implements(reflect.TypeFor[json.Marshaler]())
}

// isSentWhole reports whether a value of this type is one text in a path or a
// query, whatever its kind: one that writes its own text, or a slice that the
// server reads as one value with its UnmarshalJSON, and not element by element.
func isSentWhole(rtype reflect.Type) bool {
	return hasTextForm(rtype) || rtype.Kind() == reflect.Slice && readsJSONText(rtype)
}

// writesFromPointer reports whether a pointer type writes itself by a method of
// the pointer receiver that it has the method to read back, as *big.Int does:
// followed, such a pointer leaves a value that is written by its fields, which
// that method refuses. A type that only writes itself is read by its fields,
// and is still sent as them.
func writesFromPointer(pointer reflect.Type) bool {
	for _, form := range [][2]reflect.Type{
		{reflect.TypeFor[encoding.TextMarshaler](), reflect.TypeFor[encoding.TextUnmarshaler]()},
		{reflect.TypeFor[json.Marshaler](), reflect.TypeFor[json.Unmarshaler]()},
		{reflect.TypeFor[xml.Marshaler](), reflect.TypeFor[xml.Unmarshaler]()},
	} {
		isPair := pointer.Implements(form[0]) && pointer.Implements(form[1])
		if isPair && !pointer.Elem().Implements(form[0]) {
			return true
		}
	}
	return false
}

// wireText is the text of a value in a path or query, for the server to parse
// back: what %v prints for a type with a text form, a time.Time for one, is not
// that form. Nor is it for a string or a number whose type has a String method
// and nothing to read text with: the server reads such a value by its kind, so
// that is how it is sent.
//
// A value that its own method fails to write is an error: sent empty, it would
// count as left out, and the function would run without it. Only a zero value
// is sent empty, as left out is what it is.
func wireText(rvalue reflect.Value) (string, error) {
	text, written, err := ownText(rvalue)
	switch {
	case written:
		return text, err
	case rvalue.Kind() == reflect.Slice && readsJSONText(rvalue.Type()):
		text, err := wholeText(rvalue)
		return string(text), err
	}
	if text, ok := textByKind(rvalue); ok {
		return text, nil
	}
	return fmt.Sprintf("%v", rvalue.Interface()), nil
}

// ownText is the text a value makes of itself with its MarshalText or
// MarshalJSON method. written reports whether it has one of them.
func ownText(rvalue reflect.Value) (text string, written bool, err error) {
	switch value := rvalue.Interface().(type) {
	case encoding.TextMarshaler:
		b, err := value.MarshalText()
		if err != nil {
			return "", true, errorUnlessZero(rvalue, err)
		}
		return string(b), true, nil
	case json.Marshaler:
		b, err := value.MarshalJSON()
		if err != nil {
			return "", true, errorUnlessZero(rvalue, err)
		}
		// Read as JSON reads it: a JSON string has escapes that Go's own
		// quoting does not, a solidus for one.
		var text string
		if json.Unmarshal(b, &text) == nil {
			return text, true, nil
		}
		return string(b), true, nil
	}
	return "", false, nil
}

// textByKind is the text of a string, number or boolean as [scanKind] reads it,
// for a value that %v would print some other way, by its String, Error or
// Format method, and that the server has no method of the type to read back
// with: sent as it prints, a string would arrive as another string, and a
// number would be refused. A time.Duration is read as it prints. ok is false
// for every other value, which %v prints as it is read.
func textByKind(rvalue reflect.Value) (text string, ok bool) {
	rtype := rvalue.Type()
	if !printsByMethod(rtype) || readsOwnText(rtype) || rtype == reflect.TypeFor[time.Duration]() {
		return "", false
	}
	switch rvalue.Kind() {
	case reflect.String:
		return rvalue.String(), true
	case reflect.Bool:
		return strconv.FormatBool(rvalue.Bool()), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rvalue.Int(), 10), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(rvalue.Uint(), 10), true
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(rvalue.Float(), 'g', -1, rtype.Bits()), true
	case reflect.Complex64, reflect.Complex128:
		return strconv.FormatComplex(rvalue.Complex(), 'g', -1, rtype.Bits()), true
	}
	return "", false
}

// printsByMethod reports whether fmt prints a value of the type with a method
// of the type, and not by its kind.
func printsByMethod(rtype reflect.Type) bool {
	for _, printer := range []reflect.Type{
		reflect.TypeFor[fmt.Stringer](),
		reflect.TypeFor[error](),
		reflect.TypeFor[fmt.Formatter](),
	} {
		if rtype.Implements(printer) {
			return true
		}
	}
	return false
}

func errorUnlessZero(rvalue reflect.Value, err error) error {
	for rvalue.Kind() == reflect.Pointer && !rvalue.IsNil() {
		rvalue = rvalue.Elem()
	}
	if rvalue.IsZero() {
		return nil
	}
	return err
}

// keyText is the text of a map key, or of the key of a pair, as the name of what
// it is the key of in a response. A nil key has no text form to ask for, and one
// that cannot be written has no text. A key with no text of its own is as fmt
// prints it, which is what the readers of a response have always been sent.
func keyText(key reflect.Value) string {
	if isNilResult(key) {
		return fmt.Sprint(nil)
	}
	if text, written, _ := ownText(key); written {
		return text
	}
	return fmt.Sprintf("%v", key.Interface())
}

// pathText is the text of a value in a path, escaped. A remainder keeps its
// slashes, and only those: anything else in it could end the path.
func pathText(rvalue reflect.Value, isRemainder bool) (string, error) {
	if _, ok := pointedTo(rvalue.Type()); !ok {
		return "", errEndlessPointer
	}
	for rvalue.Kind() == reflect.Pointer && !rvalue.IsNil() && !isSentWhole(rvalue.Type()) {
		rvalue = rvalue.Elem()
	}
	var text string
	var err error
	switch {
	case isNilResult(rvalue):
		text = ""
	case isSentWhole(rvalue.Type()):
		text, err = wireText(rvalue)
	case isByteSlice(rvalue.Type()):
		text = string(rvalue.Bytes())
	case rvalue.Kind() == reflect.Slice:
		parts := make([]string, rvalue.Len())
		for i := range parts {
			if parts[i], err = pathText(rvalue.Index(i), false); err != nil {
				return "", err
			}
		}
		return strings.Join(parts, ","), nil
	default:
		text, err = wireText(rvalue)
	}
	if err != nil {
		return "", err
	}
	if !isRemainder {
		return url.PathEscape(text), nil
	}
	segments := strings.Split(text, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/"), nil
}

// isPathParameter reports whether a parameter is written into the path. One that
// is sent nowhere has a location with every bit set.
func isPathParameter(param parameter) bool {
	return param.Location != parameterInVoid && param.Location&parameterInPath != 0
}

type RequestWriter struct {
	io.Writer
	header http.Header
}

func (w RequestWriter) WriteHeader(status int) {}

func (w RequestWriter) Header() http.Header {
	return w.header
}

// errEmptyPathValue is for a path parameter with nothing to send: no route can
// carry it, as the path it makes is the path of another resource.
var errEmptyPathValue = errors.New("is empty, and a path cannot carry an empty value")

// errEmptyPathSegment is for a remainder that starts with a slash, as an
// absolute path does: the two slashes it makes in the path match no route, and
// the 404 that is the answer is no error to a DELETE.
var errEmptyPathSegment = errors.New("starts with a slash, and a path cannot carry an empty segment")

// clientWrite returns the endpoint of a call and writes its body. A sample is
// the picture of a call that was made some other way, so it shows a path value
// with nothing in it where a request would be refused.
func (op operation) clientWrite(header http.Header, path string, args []reflect.Value, body io.Writer, isSample bool) (endpoint, contentType string, err error) {
	var encoder func(http.ResponseWriter, any) error
	contentType = string(op.DefaultContentType)
	ctype, ok := contentTypes[contentType]
	switch {
	case contentType == "text/plain":
		encoder = encodeSentText
	case ok:
		encoder = ctype.Encode
	default:
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
	// mapped is the arguments of the mapping, in the order of their rules.
	var mapped []reflect.Value
	var elements []any
	//deref is needed to prevent fmt from formatting the pointer as an address.
	deref := func(index []int) reflect.Value {
		value := args[index[0]]
		_, hasEnd := pointedTo(value.Type())
		for value.Kind() == reflect.Ptr {
			if value.IsNil() {
				return reflect.Value{}
			}
			if !hasEnd {
				// A pointer to itself is not followed, there being no
				// end to it: it is for what writes it to refuse.
				return value
			}
			if len(index) == 1 && writesFromPointer(value.Type()) {
				// The pointer is the value that writes itself.
				return value
			}
			value = value.Elem()
		}
		if len(index) > 1 {
			return fieldByIndex(value, index[1:], false)
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
		if param.Location == parameterInBody && op.argumentsNeedsMapping {
			mapped = append(mapped, args[param.Index[0]])
		}
		if !value.IsValid() {
			switch {
			case isPathParameter(param) && !isSample:
				return "", "", fmt.Errorf("%s: the path parameter %q %w", op.Name, param.Name, errEmptyPathValue)
			case param.Location != parameterInBody, op.argumentsNeedsMapping:
				// Left out, which the server reads as nil.
			case op.bodyArrayLength > 0:
				elements = append(elements, nil)
			case param.optional:
				// Left out as well, there being no body to send it in.
			case contentType == "application/json":
				// A nil *T is an argument like any other, and the server
				// takes a body of null for one. Only JSON has a null to
				// send: in the other types the body is empty, which the
				// server reads as the nil it is.
				if err := encoder(writer, args[param.Index[0]].Interface()); err != nil {
					return "", "", err
				}
			}
			continue
		}
		if param.expands {
			if value.Kind() == reflect.Map {
				for entries := value.MapRange(); entries.Next(); {
					name, err := wireText(entries.Key())
					if err != nil && !isSample {
						return "", "", fmt.Errorf("%s: cannot write the name of a query parameter: %w", op.Name, err)
					}
					if err := op.encodeQueryValue(name, query, entries.Value(), true); err != nil && !isSample {
						return "", "", err
					}
				}
			}
			continue
		}
		if isPathParameter(param) {
			str, err := pathText(value, strings.HasSuffix(param.Name, "*"))
			if err != nil && !isSample {
				return "", "", fmt.Errorf("%s: cannot write the path parameter %q: %w", op.Name, param.Name, err)
			}
			if str == "" && !isSample {
				return "", "", fmt.Errorf("%s: the path parameter %q %w", op.Name, param.Name, errEmptyPathValue)
			}
			if strings.HasPrefix(str, "/") && !isSample {
				return "", "", fmt.Errorf("%s: the path parameter %q %w", op.Name, param.Name, errEmptyPathSegment)
			}
			path = strings.Replace(path, "{"+param.Name+"}", str, 1)
		}
		if param.Location&parameterInQuery != 0 {
			queryValue := value
			if len(param.Index) == 1 {
				// Keep the presence carried by a whole pointer argument. Other
				// encodings still use the dereferenced value above.
				queryValue = args[param.Index[0]]
			}
			if err := op.encodeQuery(param.Name, query, queryValue); err != nil && !isSample {
				return "", "", err
			}
		}
		if param.Location == parameterInBody {
			switch {
			case op.bodyArrayLength > 0:
				// Its own place in the array, also for a struct that the
				// path or query carries fields of: the server reads an
				// element for each body argument, and with another argument
				// to send there is a body for the struct to travel in.
				elements = append(elements, value.Interface())
			case param.optional:
				// What the body would hold is in the path or query already,
				// and the method has no body to send it in.
			case op.argumentsNeedsMapping:
				mapping[param.Name] = value.Interface()
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
		var body any = mapping
		if contentType == "application/xml" {
			body = mappedElement(op.Name, op.argMappingType, mapped)
		}
		if err := encoder(writer, body); err != nil {
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
	for _, route := range spec.everyOperation() {
		var (
			op = route.operation
			fn = op.Function
		)
		var (
			method = string(route.method) //Determine the HTTP method of this request.
			path   = rtags.CleanupPattern(route.path)
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
				endpoint, contentType, err := op.clientWrite(headers, path, args, writer, false)
				if err != nil {
					return nil, err
				}
				// The body goes with the request that opens the stream, as
				// the server reads its arguments before it starts one.
				var body io.Reader
				if writer.Len() > 0 {
					body = writer
				}
				req, err := http.NewRequestWithContext(ctx, "GET", host+endpoint, body)
				if err != nil {
					return nil, err
				}
				maps.Copy(req.Header, headers)
				if body != nil && req.Header.Get("Content-Type") == "" {
					req.Header.Set("Content-Type", contentType)
				}

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
			switch method {
			case "GET", "HEAD", "OPTIONS", "TRACE", "DELETE":
				// These methods should not have a body, unless the function
				// has an argument for one: the server does not call it
				// without, so leaving the body out only loses the argument.
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
		// Deleting is asking for a thing to be gone, and one that was not
		// there to begin with is gone: the call has what it asked for. This
		// is deliberate, and cannot tell a thing that is gone from a route
		// that the server does not have.
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
