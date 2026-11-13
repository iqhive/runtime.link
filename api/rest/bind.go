package rest

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"math/cmplx"
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
//
// A type that reads its own text is asked before its kind is looked at: that
// reading is where such a type refuses what it does not hold, and a function
// that takes one relies on never being handed anything else.
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
	if unmarshaler, ok := ref.Interface().(json.Unmarshaler); ok {
		return scanJSONText(unmarshaler, text)
	}
	if ok, err := scanTypeOf(ref, text); ok {
		return err
	}
	if scanner, ok := ref.Interface().(fmt.Scanner); ok {
		return scanAll(scanner, text)
	}
	if dst.Kind() == reflect.String {
		dst.SetString(text)
		return nil
	}
	if dst.Type() == reflect.TypeFor[time.Duration]() {
		if duration, err := time.ParseDuration(text); err == nil {
			dst.SetInt(int64(duration))
			return nil
		}
	}
	return scanKind(dst, text)
}

// scanAll has scanner read text, all of it: fmt is content with a Scan that
// stops early, and what it left unread would be dropped without a word. The
// space that ends a line of text is not a value left unread.
func scanAll(scanner fmt.Scanner, text string) error {
	unread := strings.NewReader(text)
	if _, err := fmt.Fscanf(unread, "%v", scanner); err != nil {
		return err
	}
	rest := text[len(text)-unread.Len():]
	if strings.TrimSpace(rest) != "" {
		return fmt.Errorf("cannot read %q: %q is left over", text, rest)
	}
	return nil
}

// scanJSONText tries text as the JSON value it looks like before trying it as a
// JSON string, as a query has no quotes to tell them apart.
func scanJSONText(unmarshaler json.Unmarshaler, text string) error {
	if isUnquotedJSON(text) {
		if err := unmarshaler.UnmarshalJSON([]byte(text)); err == nil {
			return nil
		}
	}
	// Quoted by encoding/json: Go's own quoting writes control characters in
	// ways JSON does not have, which would refuse text that is a fine string.
	quoted, err := json.Marshal(text)
	if err != nil {
		return err
	}
	return unmarshaler.UnmarshalJSON(quoted)
}

// isUnquotedJSON reports whether text looks like a number, a boolean, an object
// or an array: what a type's MarshalJSON may have written that the client sends
// as it is, where it takes the quotes off a string.
func isUnquotedJSON(text string) bool {
	if _, err := strconv.ParseFloat(text, 64); err == nil || text == "true" || text == "false" {
		return true
	}
	return (strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[")) && json.Valid([]byte(text))
}

// errNotFinite is for NaN and the infinities, which a function checking the
// range of a number does not expect: NaN passes every comparison it could make.
var errNotFinite = errors.New("a number is finite")

// isFinite reports whether a complex number has no NaN or infinity in it.
func isFinite(value complex128) bool {
	return !cmplx.IsNaN(value) && !cmplx.IsInf(value)
}

// scanKind parses numbers in decimal: a leading zero has no meaning to a client
// that is not writing Go, and base prefixes are not part of the API. Nor are
// NaN and the infinities, which a function checking the range of a number does
// not expect, and which no JSON body could have sent it.
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
		if err == nil && !isFinite(complex(value, 0)) {
			return fmt.Errorf("cannot read %q as %v: %w", text, dst.Type(), errNotFinite)
		}
		dst.SetFloat(value)
	case reflect.Complex64, reflect.Complex128:
		var value complex128
		value, err = strconv.ParseComplex(text, dst.Type().Bits())
		if err == nil && !isFinite(value) {
			return fmt.Errorf("cannot read %q as %v: %w", text, dst.Type(), errNotFinite)
		}
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
// newline around a number is no part of it but every byte of a string is. A
// list is read as it is written, a line to an element.
func scanBodyText(v any, text string) error {
	dst := reflect.ValueOf(v).Elem()
	rtype := dst.Type()
	for rtype.Kind() == reflect.Pointer {
		rtype = rtype.Elem()
	}
	if isListType(rtype) && !isByteSlice(rtype) {
		lines := bodyLines(text)
		if trimsBodySpace(rtype.Elem()) {
			for i, line := range lines {
				lines[i] = strings.TrimSpace(line)
			}
		}
		return scanList(dst, lines)
	}
	if trimsBodySpace(rtype) {
		text = strings.TrimSpace(text)
	}
	return scanText(dst, text)
}

// trimsBodySpace reports whether the space around a value of this type in a
// body is no part of it, which is so for all that is read by kind but a string.
func trimsBodySpace(rtype reflect.Type) bool {
	for rtype.Kind() == reflect.Pointer {
		rtype = rtype.Elem()
	}
	return isScalarKind(rtype.Kind()) && rtype.Kind() != reflect.String
}

// bodyLines splits a text/plain body into the lines a list is written as, each
// of them ended by a newline, so that the last one starts no further element.
func bodyLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// isListType reports whether a type is sent as several values: a slice, unless
// it is one value to its own text form.
func isListType(rtype reflect.Type) bool {
	return rtype.Kind() == reflect.Slice && !readsWholeText(rtype)
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

// readsOwnText reports whether a type reads its own text, by any of the ways
// [scanText] asks it to before it falls back on the type's kind.
func readsOwnText(rtype reflect.Type) bool {
	return readsWholeText(rtype) || reflect.PointerTo(rtype).Implements(reflect.TypeFor[fmt.Scanner]())
}

// parameterText returns the values sent for a path or query parameter. A list in
// a path is comma separated, the style its documentation gives it.
func parameterText(r *http.Request, query url.Values, param parameter, isList bool) []string {
	var texts []string
	if param.Location&parameterInPath != 0 {
		name := strings.TrimSuffix(param.Name, "*")
		if text := r.PathValue(name); text != "" {
			texts = []string{text}
			if isList {
				texts = pathList(r.PathValue(escapedName(name)), text)
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

// pathList splits a list in a path into its elements. A comma inside an element
// is sent escaped, so it is the escaped text that is split, and each element is
// decoded after that. Only this package's router keeps the escaped text: behind
// another one there is none, and every comma of the decoded text separates.
func pathList(escaped, text string) []string {
	if escaped == "" {
		return strings.Split(text, ",")
	}
	elements := strings.Split(escaped, ",")
	for i, element := range elements {
		decoded, err := url.PathUnescape(element)
		if err != nil {
			return strings.Split(text, ",")
		}
		elements[i] = decoded
	}
	return elements
}

// sentText returns the values a request holds for a path or query parameter of
// type rtype, and whether they are the elements of a list. A parameter that was
// sent empty holds none, as it counts as left out, except for a pointer to a
// string, which an empty value still points at.
func sentText(rtype reflect.Type, r *http.Request, query url.Values, param parameter) (texts []string, isList bool) {
	pointee := rtype
	for pointee.Kind() == reflect.Pointer {
		pointee = pointee.Elem()
	}
	// []byte in a path is that segment's bytes, there being no way to tell a
	// list of them from text. A query keeps one value per byte, as the client
	// sends it.
	isPathBytes := isByteSlice(pointee) && param.Location&parameterInPath != 0
	isList = isListType(pointee) && !isPathBytes
	texts = parameterText(r, query, param, isList)
	// An explicitly empty string still carries pointer presence. Keep empty
	// numeric parameters as omitted, as they have always been.
	isStringPointer := rtype.Kind() == reflect.Pointer && pointee.Kind() == reflect.String
	if isStringPointer && len(texts) == 0 && param.Location&parameterInQuery != 0 && len(query[param.Name]) > 0 {
		texts = []string{query[param.Name][0]}
	}
	return texts, isList
}

// bindParameter sets target, the settable argument or field of param, from the
// request. A pointer is only allocated for a value that was sent, so that an
// optional *T stays nil when it was left out, as it is when a client passes nil.
func bindParameter(target reflect.Value, r *http.Request, query url.Values, param parameter) error {
	texts, isList := sentText(target.Type(), r, query, param)
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
	elem := target.Type().Elem()
	for elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	// An empty element is left the zero value of a kind with nothing to parse
	// it from, a nil pointer for a list of pointers. A type that reads its own
	// text says for itself whether nothing is a value: left zero, the element
	// would be one it never agreed to.
	readsEmpty := readsOwnText(elem)
	for i, text := range texts {
		if text == "" && !readsEmpty {
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
		case isListType(rtype.Elem()):
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
// the structs on the way there when create is set, as their fields are the
// parameters.
func nestedArgument(arg reflect.Value, index []int, create bool) reflect.Value {
	for arg.Kind() == reflect.Pointer {
		if arg.IsNil() {
			arg.Set(reflect.New(arg.Type().Elem()))
		}
		arg = arg.Elem()
	}
	return fieldByIndex(arg, index, create)
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

// callableOn reports whether the named method can be called on v. On a nil
// pointer, only a method of the pointer receiver can be, which may well be
// written to answer for nil.
func callableOn(v reflect.Value, method string) bool {
	for v.Kind() == reflect.Interface && !v.IsNil() {
		v = v.Elem()
	}
	if v.Kind() != reflect.Pointer || !v.IsNil() {
		return true
	}
	_, onValue := v.Type().Elem().MethodByName(method)
	return !onValue
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
