// Package jrpc implements JSON-RPC codecs.
//
// A future JSON-RPC linker can use [runtime.link/api.Function.Args] for
// by-name params when the API implements [runtime.link/api.WithSource].
package jrpc

import (
	"bytes"
	"encoding"
	"encoding/json"
	"errors"
	"io"
	"math"
	"reflect"
	"strconv"
	"unicode/utf8"
	"unsafe"
)

// maxDepth is the deepest nesting encoding/json reads. It is also what keeps
// recursion on a cyclic value or a hostile document off the end of the stack.
const maxDepth = 10000

type Encoder struct {
	w io.Writer

	// fields is only set on the copy Encode works on: an Encoder itself may be
	// shared between goroutines, so it keeps no state of its own.
	fields map[reflect.Type][]field
}

//go:linkname write github.com/iqhive/runtime.link/api/jrpc.write_impl
//go:noescape
func write(w io.Writer, buf []byte) (int, error)

func write_string(w io.Writer, s string) (int, error) {
	return write(w, unsafe.Slice(unsafe.StringData(s), len(s)))
}

func write_impl(w io.Writer, buf []byte) (int, error) {
	return w.Write(buf)
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

func (e *Encoder) Encode(v any) error {
	// A value can fail halfway through, and half a message on the wire would
	// corrupt the stream for the peer, so it reaches e.w only once complete.
	var message bytes.Buffer
	buffered := Encoder{w: &message, fields: make(map[reflect.Type][]field)}
	if err := buffered.encode(reflect.ValueOf(v), 0); err != nil {
		return err
	}
	_, err := write(e.w, message.Bytes())
	return err
}

func (e *Encoder) null() error {
	_, err := write(e.w, []byte("null"))
	return err
}

func tooDeep(rvalue reflect.Value) error {
	return &json.UnsupportedValueError{Value: rvalue, Str: "exceeded max depth"}
}

// encode writes rvalue, which is nested inside depth arrays and objects.
// Pointers are followed in a loop because a cycle of them adds no nesting.
func (e *Encoder) encode(rvalue reflect.Value, depth int) error {
	if !rvalue.IsValid() {
		return e.null()
	}
	for range maxDepth {
		if marshaled, err := e.marshal(rvalue); marshaled {
			return err
		}
		if !isIndirect(rvalue) {
			return e.value(rvalue, depth)
		}
		if rvalue.IsNil() {
			return e.null()
		}
		rvalue = rvalue.Elem()
	}
	return tooDeep(rvalue)
}

func isIndirect(rvalue reflect.Value) bool {
	return rvalue.Kind() == reflect.Interface || rvalue.Kind() == reflect.Pointer
}

// marshal uses the value's own MarshalJSON or MarshalText, reporting whether it had one.
func (e *Encoder) marshal(rvalue reflect.Value) (bool, error) {
	marshaler, textMarshaler := marshalerOf(rvalue)
	if marshaler != nil {
		out, err := marshaler.MarshalJSON()
		if err != nil {
			return true, &json.MarshalerError{Type: rvalue.Type(), Err: err}
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, out); err != nil {
			return true, &json.MarshalerError{Type: rvalue.Type(), Err: err}
		}
		_, err = write(e.w, compact.Bytes())
		return true, err
	}
	if textMarshaler != nil {
		out, err := textMarshaler.MarshalText()
		if err != nil {
			return true, &json.MarshalerError{Type: rvalue.Type(), Err: err}
		}
		return true, e.string(string(out))
	}
	return false, nil
}

// marshalerOf picks the method encoding/json would call: JSON before text, and
// a pointer receiver only when rvalue is addressable.
func marshalerOf(rvalue reflect.Value) (json.Marshaler, encoding.TextMarshaler) {
	if isNilReceiver(rvalue) || !rvalue.CanInterface() {
		return nil, nil
	}
	rtype := rvalue.Type()
	addressable := rtype.Kind() != reflect.Pointer && rvalue.CanAddr()
	marshaler := reflect.TypeFor[json.Marshaler]()
	if addressable && reflect.PointerTo(rtype).Implements(marshaler) {
		method, _ := rvalue.Addr().Interface().(json.Marshaler)
		return method, nil
	}
	if rtype.Implements(marshaler) {
		method, _ := rvalue.Interface().(json.Marshaler)
		return method, nil
	}
	textMarshaler := reflect.TypeFor[encoding.TextMarshaler]()
	if addressable && reflect.PointerTo(rtype).Implements(textMarshaler) {
		method, _ := rvalue.Addr().Interface().(encoding.TextMarshaler)
		return nil, method
	}
	if rtype.Implements(textMarshaler) {
		method, _ := rvalue.Interface().(encoding.TextMarshaler)
		return nil, method
	}
	return nil, nil
}

// isNilReceiver includes an interface around a nil pointer, on which calling a
// value method panics. Leaving it to encode writes null, as for any nil pointer.
func isNilReceiver(rvalue reflect.Value) bool {
	if !isIndirect(rvalue) {
		return false
	}
	if rvalue.IsNil() {
		return true
	}
	return rvalue.Kind() == reflect.Interface && rvalue.Elem().Kind() == reflect.Pointer && rvalue.Elem().IsNil()
}

func (e *Encoder) value(rvalue reflect.Value, depth int) error {
	rtype := rvalue.Type()
	switch rtype.Kind() {
	case reflect.Bool:
		if rvalue.Bool() {
			if _, err := write(e.w, []byte("true")); err != nil {
				return err
			}
		} else {
			if _, err := write(e.w, []byte("false")); err != nil {
				return err
			}
		}
	case reflect.Int32, reflect.Int16, reflect.Int8:
		var raw [11]byte
		buf := strconv.AppendInt(raw[:0:len(raw)], rvalue.Int(), 10)
		if _, err := write(e.w, buf); err != nil {
			return err
		}
	case reflect.Int, reflect.Int64:
		var raw [20]byte
		buf := strconv.AppendInt(raw[:0:len(raw)], rvalue.Int(), 10)
		if _, err := write(e.w, buf); err != nil {
			return err
		}
	case reflect.Uint32, reflect.Uint16, reflect.Uint8:
		var raw [11]byte
		buf := strconv.AppendUint(raw[:0:len(raw)], rvalue.Uint(), 10)
		if _, err := write(e.w, buf); err != nil {
			return err
		}
	case reflect.Uint, reflect.Uint64, reflect.Uintptr:
		var raw [20]byte
		buf := strconv.AppendUint(raw[:0:len(raw)], rvalue.Uint(), 10)
		if _, err := write(e.w, buf); err != nil {
			return err
		}
	case reflect.Float32, reflect.Float64:
		return e.float(rvalue)
	case reflect.String:
		if rtype == reflect.TypeFor[json.Number]() {
			return e.number(rvalue.String())
		}
		if err := e.string(rvalue.String()); err != nil {
			return err
		}
	case reflect.Array, reflect.Slice:
		if rtype.Kind() == reflect.Slice && rvalue.IsNil() {
			return e.null()
		}
		if isByteSlice(rtype) {
			return e.base64(rvalue.Bytes())
		}
		if depth == maxDepth {
			return tooDeep(rvalue)
		}
		if _, err := write(e.w, []byte{'['}); err != nil {
			return err
		}
		for i := range rvalue.Len() {
			if err := e.encode(rvalue.Index(i), depth+1); err != nil {
				return err
			}
			if i < rvalue.Len()-1 {
				if _, err := write(e.w, []byte{','}); err != nil {
					return err
				}
			}
		}
		if _, err := write(e.w, []byte{']'}); err != nil {
			return err
		}
	case reflect.Map:
		if !isEncodableMapKey(rtype.Key()) {
			return &json.UnsupportedTypeError{Type: rtype}
		}
		if rvalue.IsNil() {
			return e.null()
		}
		if depth == maxDepth {
			return tooDeep(rvalue)
		}
		return encode_map_noescape(e, rvalue, depth)
	case reflect.Struct:
		if depth == maxDepth {
			return tooDeep(rvalue)
		}
		return e.structure(rvalue, depth)
	default:
		return &json.UnsupportedTypeError{Type: rtype}
	}
	return nil
}

func (e *Encoder) structure(rvalue reflect.Value, depth int) error {
	if _, err := write(e.w, []byte{'{'}); err != nil {
		return err
	}
	first := true
	fields := layout(e.fields, rvalue.Type())
	for i := range fields {
		f := &fields[i]
		value, ok := f.valueIn(rvalue)
		if !ok || f.omitZero && isZeroValue(value) || f.omitEmpty && isEmptyValue(value) {
			continue
		}
		if !first {
			if _, err := write(e.w, []byte{','}); err != nil {
				return err
			}
		}
		first = false
		if err := e.string(f.name); err != nil {
			return err
		}
		if _, err := write(e.w, []byte{':'}); err != nil {
			return err
		}
		if err := e.member(value, f.quoted, depth+1); err != nil {
			return err
		}
	}
	_, err := write(e.w, []byte{'}'})
	return err
}

func (e *Encoder) member(value reflect.Value, quoted bool, depth int) error {
	if quoted {
		return e.quoted(value, depth)
	}
	return e.encode(value, depth)
}

// quoted writes a ,string field: the JSON text of a scalar, inside a JSON
// string. A nil pointer and a type with its own marshaler stay as they are.
func (e *Encoder) quoted(rvalue reflect.Value, depth int) error {
	if rvalue.Kind() == reflect.Pointer && !rvalue.IsNil() && !hasMarshaler(rvalue) {
		rvalue = rvalue.Elem()
	}
	if rvalue.Kind() == reflect.Pointer || hasMarshaler(rvalue) {
		return e.encode(rvalue, depth)
	}
	var text bytes.Buffer
	inner := Encoder{w: &text, fields: e.fields}
	if err := inner.encode(rvalue, depth); err != nil {
		return err
	}
	return e.string(text.String())
}

func hasMarshaler(rvalue reflect.Value) bool {
	marshaler, textMarshaler := marshalerOf(rvalue)
	return marshaler != nil || textMarshaler != nil
}

// float formats the way ES6 does, as encoding/json does, at the value's own
// precision: a float32 printed as a float64 grows spurious digits.
func (e *Encoder) float(rvalue reflect.Value) error {
	value := rvalue.Float()
	bits := rvalue.Type().Bits()
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return &json.UnsupportedValueError{Value: rvalue, Str: strconv.FormatFloat(value, 'g', -1, bits)}
	}
	format := byte('f')
	if isExponentRange(value, bits) {
		format = 'e'
	}
	var raw [32]byte
	buf := strconv.AppendFloat(raw[:0], value, format, -1, bits)
	if n := len(buf); format == 'e' && n >= 4 && buf[n-4] == 'e' && (buf[n-3] == '-' || buf[n-3] == '+') && buf[n-2] == '0' {
		buf[n-2] = buf[n-1]
		buf = buf[:n-1]
	}
	_, err := write(e.w, buf)
	return err
}

func isExponentRange(value float64, bits int) bool {
	magnitude := math.Abs(value)
	if magnitude == 0 {
		return false
	}
	if bits == 32 {
		return float32(magnitude) < 1e-6 || float32(magnitude) >= 1e21
	}
	return magnitude < 1e-6 || magnitude >= 1e21
}

// number writes a json.Number as the number it holds. The empty Number is its
// zero value, which encoding/json writes as 0.
func (e *Encoder) number(number string) error {
	if number == "" {
		number = "0"
	}
	if !isNumberLiteral(number) {
		return errors.New("jrpc: invalid number literal " + strconv.Quote(number))
	}
	_, err := write_string(e.w, number)
	return err
}

// isNumberLiteral relies on a valid JSON text that starts and ends like a
// number having to be one, without any whitespace around it.
func isNumberLiteral(number string) bool {
	first, last := number[0], number[len(number)-1]
	return (first == '-' || isDigit(first)) && isDigit(last) && json.Valid([]byte(number))
}

func isDigit(char byte) bool {
	return char >= '0' && char <= '9'
}

// isByteSlice leaves out element types with marshalers: their methods would be skipped.
func isByteSlice(rtype reflect.Type) bool {
	if rtype.Kind() != reflect.Slice || rtype.Elem().Kind() != reflect.Uint8 {
		return false
	}
	pointer := reflect.PointerTo(rtype.Elem())
	return !pointer.Implements(reflect.TypeFor[json.Marshaler]()) &&
		!pointer.Implements(reflect.TypeFor[encoding.TextMarshaler]())
}

func isEncodableMapKey(rtype reflect.Type) bool {
	switch rtype.Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	}
	return rtype.Implements(reflect.TypeFor[encoding.TextMarshaler]())
}

func isEmptyValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return value.Len() == 0
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Interface, reflect.Pointer:
		return value.IsZero()
	}
	return false
}

// isZeroValue honours an IsZero method, which encoding/json prefers to the
// zero value, and treats a nil receiver as zero rather than calling it.
func isZeroValue(value reflect.Value) bool {
	rtype := value.Type()
	method := reflect.TypeFor[zeroer]()
	switch {
	case !value.CanInterface():
		return value.IsZero()
	case rtype.Implements(method) && isNilReceiver(value):
		return true
	case rtype.Implements(method):
		zero, _ := value.Interface().(zeroer)
		return zero.IsZero()
	case !isIndirect(value) && reflect.PointerTo(rtype).Implements(method):
		if !value.CanAddr() {
			addressable := reflect.New(rtype).Elem()
			addressable.Set(value)
			value = addressable
		}
		zero, _ := value.Addr().Interface().(zeroer)
		return zero.IsZero()
	}
	return value.IsZero()
}

//go:linkname encode_map_noescape github.com/iqhive/runtime.link/api/jrpc.encode_map
//go:noescape
func encode_map_noescape(e *Encoder, rvalue reflect.Value, depth int) error

// encode_map reads entries with iter.Key and iter.Value because map elements
// are not addressable: encoding/json does not call pointer receivers on them,
// and a method handed the address of a stack buffer could keep it.
func encode_map(e *Encoder, rvalue reflect.Value, depth int) error {
	if _, err := write(e.w, []byte{'{'}); err != nil {
		return err
	}
	iter := rvalue.MapRange()
	first := true
	for iter.Next() {
		if !first {
			if _, err := write(e.w, []byte{','}); err != nil {
				return err
			}
		}
		first = false
		name, err := mapKeyName(iter.Key())
		if err != nil {
			return &json.MarshalerError{Type: rvalue.Type(), Err: err}
		}
		if err := e.string(name); err != nil {
			return err
		}
		if _, err := write(e.w, []byte{':'}); err != nil {
			return err
		}
		if err := e.encode(iter.Value(), depth+1); err != nil {
			return err
		}
	}
	if _, err := write(e.w, []byte{'}'}); err != nil {
		return err
	}
	return nil
}

// mapKeyName gives a key its JSON name, which is always a string. A string
// kind wins over MarshalText, and a nil pointer key is the empty name. A nil
// interface key has no type to go by; encoding/json panics on it.
func mapKeyName(key reflect.Value) (string, error) {
	if key.Kind() == reflect.String {
		return key.String(), nil
	}
	if key.Kind() == reflect.Interface && key.IsNil() {
		return "", errors.New("jrpc: nil map key")
	}
	if isNilReceiver(key) {
		return "", nil
	}
	if marshaler, ok := key.Interface().(encoding.TextMarshaler); ok {
		text, err := marshaler.MarshalText()
		return string(text), err
	}
	switch key.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(key.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(key.Uint(), 10), nil
	}
	return "", errors.New("jrpc: unsupported map key: " + key.Type().String())
}

// string is ported from encoding/json
// support has been dropped for htmlEscape.
func (e *Encoder) string(s string) error {
	const hex = "0123456789abcdef"
	if _, err := write(e.w, []byte{'"'}); err != nil {
		return err
	}
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if !(b <= 31 || b == '"' || b == '\\') {
				i++
				continue
			}
			if start < i {
				if _, err := write_string(e.w, s[start:i]); err != nil {
					return err
				}
			}
			if _, err := write(e.w, []byte{'\\'}); err != nil {
				return err
			}
			switch b {
			case '\\', '"':
				if _, err := write(e.w, []byte{b}); err != nil {
					return err
				}
			case '\n':
				if _, err := write(e.w, []byte{'n'}); err != nil {
					return err
				}
			case '\r':
				if _, err := write(e.w, []byte{'r'}); err != nil {
					return err
				}
			case '\t':
				if _, err := write(e.w, []byte{'t'}); err != nil {
					return err
				}
			default:
				// This encodes bytes < 0x20 except for \t, \n and \r.
				// If escapeHTML is set, it also escapes <, >, and &
				// because they can lead to security holes when
				// user-controlled strings are rendered into JSON
				// and served to some browsers.
				if _, err := write_string(e.w, `u00`); err != nil {
					return err
				}
				if _, err := write(e.w, []byte{hex[b>>4]}); err != nil {
					return err
				}
				if _, err := write(e.w, []byte{hex[b&0xF]}); err != nil {
					return err
				}
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				if _, err := write_string(e.w, s[start:i]); err != nil {
					return err
				}
			}
			if _, err := write_string(e.w, `\ufffd`); err != nil {
				return err
			}
			i += size
			start = i
			continue
		}
		// U+2028 is LINE SEPARATOR.
		// U+2029 is PARAGRAPH SEPARATOR.
		// They are both technically valid characters in JSON strings,
		// but don't work in JSONP, which has to be evaluated as JavaScript,
		// and can lead to security holes there. It is valid JSON to
		// escape them, so we do so unconditionally.
		// See http://timelessrepo.com/json-isnt-a-javascript-subset for discussion.
		if c == '\u2028' || c == '\u2029' {
			if start < i {
				if _, err := write_string(e.w, s[start:i]); err != nil {
					return err
				}
			}
			if _, err := write_string(e.w, `\u202`); err != nil {
				return err
			}
			if _, err := write(e.w, []byte{hex[c&0xF]}); err != nil {
				return err
			}
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		if _, err := write_string(e.w, s[start:]); err != nil {
			return err
		}
	}
	if _, err := write(e.w, []byte{'"'}); err != nil {
		return err
	}
	return nil
}

// base64 is an efficient base64 encoder, adapted from encoding/base64.
func (m *Encoder) base64(b []byte) error {
	const std = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	if len(b) == 0 {
		_, err := write_string(m.w, `""`)
		return err
	}
	if _, err := write(m.w, []byte{'"'}); err != nil {
		return err
	}
	// Encode full quanta.
	n := len(b) / 3 * 3
	for i := 0; i < n; i += 3 {
		// We could use binary.BigEndian.Uint32 here, but it's overkill.
		x := uint32(b[i+0])<<16 | uint32(b[i+1])<<8 | uint32(b[i+2])
		if _, err := write(m.w, []byte{std[x>>18]}); err != nil {
			return err
		}
		if _, err := write(m.w, []byte{std[x>>12&0x3f]}); err != nil {
			return err
		}
		if _, err := write(m.w, []byte{std[x>>6&0x3f]}); err != nil {
			return err
		}
		if _, err := write(m.w, []byte{std[x&0x3f]}); err != nil {
			return err
		}
	}
	remain := len(b) - n
	if remain == 0 {
		if _, err := write(m.w, []byte{'"'}); err != nil {
			return err
		}
		return nil
	}
	// Encode partial quanta.
	switch len(b) - n {
	case 1:
		x := uint32(b[n+0])
		if _, err := write(m.w, []byte{std[x>>2]}); err != nil {
			return err
		}
		if _, err := write(m.w, []byte{std[x<<4&0x3f]}); err != nil {
			return err
		}
		if _, err := write_string(m.w, "=="); err != nil {
			return err
		}
	case 2:
		x := uint32(b[n+0])<<8 | uint32(b[n+1])
		if _, err := write(m.w, []byte{std[x>>10]}); err != nil {
			return err
		}
		if _, err := write(m.w, []byte{std[x>>4&0x3f]}); err != nil {
			return err
		}
		if _, err := write(m.w, []byte{std[x<<2&0x3f]}); err != nil {
			return err
		}
		if _, err := write(m.w, []byte{'='}); err != nil {
			return err
		}
	}
	if _, err := write(m.w, []byte{'"'}); err != nil {
		return err
	}
	return nil
}
