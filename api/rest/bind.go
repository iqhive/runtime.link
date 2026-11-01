package rest

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/iqhive/runtime.link/api"
	http_api "github.com/iqhive/runtime.link/api/internal/http"
)

// maxDecodedBody caps a request body the transport decodes into arguments, all
// of which is held in memory. A body the function reads itself is not capped.
const maxDecodedBody = 32 << 20

var errBodyArrayLength = errors.New("the body array does not hold one element per argument")

// scanText sets dst, which must be settable, from a path, query or text/plain
// value. Every form of value here is read in full, so that what is not a valid
// value is refused instead of being cut short at the first odd character.
func scanText(dst reflect.Value, text string) error {
	for dst.Kind() == reflect.Pointer {
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		dst = dst.Elem()
	}
	ref := dst.Addr()
	if unmarshaler, ok := ref.Interface().(encoding.TextUnmarshaler); ok {
		return unmarshaler.UnmarshalText([]byte(text))
	}
	if dst.Kind() == reflect.String {
		dst.SetString(text)
		return nil
	}
	if unmarshaler, ok := ref.Interface().(json.Unmarshaler); ok {
		return scanJSONText(unmarshaler, text)
	}
	if ok, err := scanTypeOf(ref, text); ok {
		return err
	}
	if _, ok := ref.Interface().(fmt.Scanner); ok {
		_, err := fmt.Sscanf(text, "%v", ref.Interface())
		return err
	}
	if dst.Type() == reflect.TypeFor[time.Duration]() {
		if duration, err := time.ParseDuration(text); err == nil {
			dst.SetInt(int64(duration))
			return nil
		}
	}
	return scanKind(dst, text)
}

// scanJSONText tries text as the JSON value it looks like before trying it as a
// JSON string, as a query has no quotes to tell them apart.
func scanJSONText(unmarshaler json.Unmarshaler, text string) error {
	if _, err := strconv.ParseFloat(text, 64); err == nil || text == "true" || text == "false" {
		if err := unmarshaler.UnmarshalJSON([]byte(text)); err == nil {
			return nil
		}
	}
	return unmarshaler.UnmarshalJSON([]byte(strconv.Quote(text)))
}

// scanKind parses numbers in decimal: a leading zero has no meaning to a client
// that is not writing Go, and base prefixes are not part of the API.
func scanKind(dst reflect.Value, text string) error {
	var err error
	switch dst.Kind() {
	case reflect.Bool:
		var value bool
		value, err = strconv.ParseBool(text)
		dst.SetBool(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var value int64
		value, err = strconv.ParseInt(text, 10, dst.Type().Bits())
		dst.SetInt(value)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		var value uint64
		value, err = strconv.ParseUint(text, 10, dst.Type().Bits())
		dst.SetUint(value)
	case reflect.Float32, reflect.Float64:
		var value float64
		value, err = strconv.ParseFloat(text, dst.Type().Bits())
		dst.SetFloat(value)
	case reflect.Complex64, reflect.Complex128:
		var value complex128
		value, err = strconv.ParseComplex(text, dst.Type().Bits())
		dst.SetComplex(value)
	case reflect.Slice:
		if !isByteSlice(dst.Type()) {
			return fmt.Errorf("cannot read %v from text", dst.Type())
		}
		dst.SetBytes([]byte(text))
	case reflect.Interface:
		if dst.NumMethod() != 0 {
			return fmt.Errorf("cannot read %v from text", dst.Type())
		}
		dst.Set(reflect.ValueOf(text))
	default:
		return fmt.Errorf("cannot read %v from text", dst.Type())
	}
	if err != nil {
		// Flattened, as [api.TransportError] would describe a *strconv.NumError
		// to the client as a number, which this may not be.
		return fmt.Errorf("cannot read %q as %v: %v", text, dst.Type(), err)
	}
	return nil
}

func isByteSlice(rtype reflect.Type) bool {
	return rtype.Kind() == reflect.Slice && rtype.Elem().Kind() == reflect.Uint8
}

// scanBodyText sets the value v points to from a text/plain body, where a final
// newline around a number is no part of it but every byte of a string is.
func scanBodyText(v any, text string) error {
	dst := reflect.ValueOf(v).Elem()
	rtype := dst.Type()
	for rtype.Kind() == reflect.Pointer {
		rtype = rtype.Elem()
	}
	if isScalarKind(rtype.Kind()) && rtype.Kind() != reflect.String {
		text = strings.TrimSpace(text)
	}
	return scanText(dst, text)
}

// isScalarKind reports whether a value of this kind holds no other values.
func isScalarKind(kind reflect.Kind) bool {
	return kind >= reflect.Bool && kind <= reflect.Complex128 || kind == reflect.String
}

// readsWholeText reports whether a slice type is one value to its text form, as
// net.IP is, and not a list of them.
func readsWholeText(rtype reflect.Type) bool {
	pointer := reflect.PointerTo(rtype)
	return pointer.Implements(reflect.TypeFor[encoding.TextUnmarshaler]()) || pointer.Implements(reflect.TypeFor[json.Unmarshaler]())
}

// parameterText returns the values sent for a path or query parameter. A list in
// a path is comma separated, the style its documentation gives it.
func parameterText(r *http.Request, query url.Values, param parameter, isList bool) []string {
	var texts []string
	if param.Location&parameterInPath != 0 {
		if text := r.PathValue(strings.TrimSuffix(param.Name, "*")); text != "" {
			texts = []string{text}
			if isList {
				texts = strings.Split(text, ",")
			}
		}
	}
	if param.Location&parameterInQuery != 0 {
		switch {
		case !isList:
			if text := query.Get(param.Name); text != "" {
				texts = []string{text}
			}
		case len(query[param.Name+"[]"]) > 0:
			texts = query[param.Name+"[]"]
		case len(query[param.Name]) > 0:
			texts = query[param.Name]
		}
	}
	return texts
}

// bindParameter sets target, the settable argument or field of param, from the
// request. A pointer is only allocated for a value that was sent, so that an
// optional *T stays nil when it was left out, as it is when a client passes nil.
func bindParameter(target reflect.Value, r *http.Request, query url.Values, param parameter) error {
	rtype := target.Type()
	for rtype.Kind() == reflect.Pointer {
		rtype = rtype.Elem()
	}
	// []byte in a path is that segment's bytes, there being no way to tell a
	// list of them from text. A query keeps one value per byte, as the client
	// sends it.
	isPathBytes := isByteSlice(rtype) && param.Location&parameterInPath != 0
	isList := rtype.Kind() == reflect.Slice && !readsWholeText(rtype) && !isPathBytes
	texts := parameterText(r, query, param, isList)
	if !isList {
		if len(texts) == 0 {
			return nil
		}
		return scanText(target, texts[0])
	}
	if target.Kind() == reflect.Pointer && len(texts) == 0 {
		return nil
	}
	return scanList(target, texts)
}

// scanList fills a slice, or a pointer to one, element by element.
func scanList(target reflect.Value, texts []string) error {
	for target.Kind() == reflect.Pointer {
		if target.IsNil() {
			target.Set(reflect.New(target.Type().Elem()))
		}
		target = target.Elem()
	}
	target.Set(reflect.MakeSlice(target.Type(), len(texts), len(texts)))
	for i, text := range texts {
		if text == "" {
			continue
		}
		if err := scanText(target.Index(i), text); err != nil {
			return err
		}
	}
	return nil
}

// expandQuery fills a map with the query parameters no other parameter names.
func expandQuery(target reflect.Value, query url.Values, op operation) error {
	rtype := target.Type()
	target.Set(reflect.MakeMap(rtype))
	for name, values := range query {
		if op.namesQueryParameter(name) {
			continue
		}
		key := reflect.New(rtype.Key()).Elem()
		if err := scanText(key, name); err != nil {
			return &api.TransportError{Name: name, Err: err}
		}
		value := reflect.New(rtype.Elem()).Elem()
		var err error
		switch {
		case rtype.Elem().Kind() == reflect.Slice && !readsWholeText(rtype.Elem()):
			err = scanList(value, values)
		default:
			err = scanText(value, values[0])
		}
		if err != nil {
			return &api.TransportError{Name: name, Err: err}
		}
		target.SetMapIndex(key, value)
	}
	return nil
}

func (op operation) namesQueryParameter(name string) bool {
	if _, constant := op.Constants[name]; constant {
		return true
	}
	return slices.ContainsFunc(op.Parameters, func(param parameter) bool {
		isQuery := param.Location != parameterInVoid && param.Location&parameterInQuery != 0
		return isQuery && !param.expands && (param.Name == name || param.Name+"[]" == name)
	})
}

// nestedArgument returns the field of an argument that index leads to, creating
// the structs on the way there, as their fields are the parameters.
func nestedArgument(arg reflect.Value, index []int) reflect.Value {
	for arg.Kind() == reflect.Pointer {
		if arg.IsNil() {
			arg.Set(reflect.New(arg.Type().Elem()))
		}
		arg = arg.Elem()
	}
	return fieldByIndex(arg, index)
}

// headerReaderOf returns the header reader of an argument, whose method is often
// on the pointer receiver, unless the argument is an optional that was left out.
func headerReaderOf(arg reflect.Value) (http_api.HeaderReader, bool) {
	if arg.Kind() != reflect.Pointer {
		reader, ok := arg.Addr().Interface().(http_api.HeaderReader)
		return reader, ok
	}
	if arg.IsNil() {
		return nil, false
	}
	reader, ok := arg.Interface().(http_api.HeaderReader)
	return reader, ok
}

// isNilResult reports whether calling a method on v would dereference nothing.
func isNilResult(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	}
	return false
}
