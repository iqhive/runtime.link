package jrpc

import (
	"encoding"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strconv"
	_ "unsafe"
)

type Decoder struct {
	r io.Reader

	segment [256]byte
	readErr error

	head, size int

	// depth counts the objects and arrays around the current value.
	depth int
	// mismatch is the first error that left the stream readable. It is
	// returned once the value that caused it has been read to its end.
	mismatch error
	// failed ends the stream: after a syntax or read error the position inside
	// the document is lost, and what follows must not pass for a new message.
	failed error

	fields map[reflect.Type][]field
}

func NewDecoder(r io.Reader) Decoder {
	return Decoder{r: r, fields: make(map[reflect.Type][]field)}
}

//go:linkname read github.com/iqhive/runtime.link/api/jrpc.read_impl
//go:noescape
func read(w io.Reader, buf []byte) (int, error)

func read_impl(r io.Reader, buf []byte) (int, error) {
	return r.Read(buf)
}

func (d *Decoder) Decode(v any) error {
	rvalue := reflect.ValueOf(v)
	if rvalue.Kind() != reflect.Ptr || rvalue.IsNil() {
		return errors.New("jrpc: Decode requires a non-nil pointer")
	}
	if d.failed != nil {
		return d.failed
	}
	// io.EOF means the stream ended between values, and nowhere else: a caller
	// looping until EOF must not mistake a cut off message for a clean end.
	if err := d.skipWhitespace(); err != nil {
		d.failed = err
		return err
	}
	d.depth, d.mismatch = 0, nil
	if err := d.decode(rvalue.Elem()); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		d.failed = err
		return err
	}
	return d.mismatch
}

func (d *Decoder) peek_byte() (byte, error) {
	for emptyReads := 0; d.head == d.size; emptyReads++ {
		if d.readErr != nil {
			return 0, d.readErr
		}
		if emptyReads == 100 {
			return 0, io.ErrNoProgress
		}
		d.head = 0
		d.size, d.readErr = read(d.r, d.segment[:])
	}
	return d.segment[d.head], nil
}

func (d *Decoder) scan_byte() (byte, error) {
	b, err := d.peek_byte()
	if err != nil {
		return 0, err
	}
	d.head++
	return b, nil
}

func (d *Decoder) skipWhitespace() error {
	for {
		char, err := d.peek_byte()
		if err != nil {
			return err
		}
		if char != ' ' && char != '\t' && char != '\n' && char != '\r' {
			return nil
		}
		if _, err := d.scan_byte(); err != nil {
			return err
		}
	}

}

// decode dispatches on the JSON token, not the Go kind, so that null, any and
// custom unmarshalers need no case of their own. Objects and arrays are walked
// here to keep them streaming; every other value goes to encoding/json whole.
func (d *Decoder) decode(rvalue reflect.Value) error {
	if err := d.skipWhitespace(); err != nil {
		return err
	}
	if isEndless(rvalue) {
		d.keep(errors.New("jrpc: cannot decode into endless pointers: " + rvalue.Type().String()))
		return d.skipValue()
	}
	char, err := d.peek_byte()
	if err != nil {
		return err
	}
	if char == '{' || char == '[' {
		target := container(rvalue)
		if char == '{' && target.Kind() == reflect.Map && !isDecodableMapKey(target.Type().Key()) {
			d.keep(&json.UnmarshalTypeError{Value: "object", Type: target.Type()})
			return d.skipValue()
		}
		if char == '{' && (target.Kind() == reflect.Map || target.Kind() == reflect.Struct) {
			return d.object(target)
		}
		if char == '[' && (target.Kind() == reflect.Array || target.Kind() == reflect.Slice) {
			return d.array(target)
		}
	}
	raw, err := d.rawValue()
	if err != nil {
		return err
	}
	if !rvalue.CanInterface() {
		// A tagged embedded struct of an unexported type: reflect can reach its
		// exported fields, which the walkers do, but not the value itself.
		if string(raw) != "null" {
			d.keep(&json.UnmarshalTypeError{Value: "value", Type: rvalue.Type()})
		}
		return nil
	}
	// raw is well formed, so what fails here is a value that does not suit its
	// target, which encoding/json reports only after finishing the document.
	d.keep(json.Unmarshal(raw, rvalue.Addr().Interface()))
	return nil
}

// isEndless finds targets with no end to their pointers, which are cycles in
// the value or a type like `type P *P`. Depending on the toolchain,
// encoding/json hangs on these or dies of a stack overflow nothing can recover.
func isEndless(rvalue reflect.Value) bool {
	rtype := rvalue.Type()
	for range maxDepth {
		switch {
		case rvalue.IsValid() && isIndirect(rvalue) && !rvalue.IsNil():
			rvalue = rvalue.Elem()
			rtype = rvalue.Type()
		case rtype.Kind() == reflect.Pointer:
			rvalue = reflect.Value{}
			rtype = rtype.Elem()
		default:
			return false
		}
	}
	return true
}

// keep remembers the first error that leaves the stream readable.
func (d *Decoder) keep(err error) {
	if d.mismatch == nil {
		d.mismatch = err
	}
}

func exceededMaxDepth() error {
	return errors.New("jrpc: exceeded max depth")
}

// container finds the value an object or array decodes into, as encoding/json
// does: through pointers, allocating them, and through interfaces only when
// they hold a pointer. The zero Value leaves the decision to encoding/json,
// because the target has its own decoding or is not a container at all.
// isEndless has already ruled out a chain of pointers that would not stop.
func container(rvalue reflect.Value) reflect.Value {
	for {
		if hasUnmarshaler(rvalue.Type()) {
			return reflect.Value{}
		}
		switch rvalue.Kind() {
		case reflect.Interface:
			if !holdsPointer(rvalue) {
				return reflect.Value{}
			}
			rvalue = rvalue.Elem()
		case reflect.Pointer:
			if rvalue.IsNil() && !rvalue.CanSet() {
				return reflect.Value{}
			}
			if rvalue.IsNil() {
				rvalue.Set(reflect.New(rvalue.Type().Elem()))
			}
			rvalue = rvalue.Elem()
		default:
			return rvalue
		}
	}
}

func holdsPointer(rvalue reflect.Value) bool {
	return !rvalue.IsNil() && rvalue.Elem().Kind() == reflect.Pointer && !rvalue.Elem().IsNil()
}

func hasUnmarshaler(rtype reflect.Type) bool {
	pointer := reflect.PointerTo(rtype)
	return pointer.Implements(reflect.TypeFor[json.Unmarshaler]()) ||
		pointer.Implements(reflect.TypeFor[encoding.TextUnmarshaler]())
}

// isDecodableMapKey is settled here and not left to encoding/json because
// go1.27 accepts more key kinds than go1.25 and than the encoder will write.
func isDecodableMapKey(rtype reflect.Type) bool {
	switch rtype.Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	}
	return reflect.PointerTo(rtype).Implements(reflect.TypeFor[encoding.TextUnmarshaler]())
}

// object decodes the object at the head of the stream into a struct or a map.
func (d *Decoder) object(target reflect.Value) error {
	d.head++
	d.depth++
	if d.depth > maxDepth {
		return exceededMaxDepth()
	}
	if target.Kind() == reflect.Map && target.IsNil() {
		target.Set(reflect.MakeMap(target.Type()))
	}
	if err := d.skipWhitespace(); err != nil {
		return err
	}
	char, err := d.peek_byte()
	if err != nil {
		return err
	}
	if char == '}' {
		d.head++
		d.depth--
		return nil
	}
	var fields []field
	if target.Kind() == reflect.Struct {
		fields = layout(d.fields, target.Type())
	}
	for {
		raw, key, err := d.objectKey()
		if err != nil {
			return err
		}
		if err := d.member(target, fields, raw, key); err != nil {
			return err
		}
		if err := d.skipWhitespace(); err != nil {
			return err
		}
		char, err := d.scan_byte()
		if err != nil {
			return err
		}
		if char == '}' {
			d.depth--
			return nil
		}
		if char != ',' {
			return errors.New("expected comma or closing brace after object value")
		}
	}
}

// objectKey consumes a key and its colon, and returns the key both as JSON
// text and as the string it spells.
func (d *Decoder) objectKey() ([]byte, string, error) {
	if err := d.skipWhitespace(); err != nil {
		return nil, "", err
	}
	char, err := d.peek_byte()
	if err != nil {
		return nil, "", err
	}
	if char != '"' {
		return nil, "", errors.New("expected string as object key")
	}
	raw, err := d.rawValue()
	if err != nil {
		return nil, "", err
	}
	var key string
	if err := json.Unmarshal(raw, &key); err != nil {
		return nil, "", err
	}
	if err := d.skipWhitespace(); err != nil {
		return nil, "", err
	}
	char, err = d.scan_byte()
	if err != nil {
		return nil, "", err
	}
	if char != ':' {
		return nil, "", errors.New("expected colon after object key")
	}
	return raw, key, nil
}

func (d *Decoder) member(target reflect.Value, fields []field, raw []byte, key string) error {
	if target.Kind() == reflect.Struct {
		return d.structMember(target, matchField(fields, key))
	}
	return d.mapEntry(target, raw, key)
}

func (d *Decoder) structMember(target reflect.Value, member *field) error {
	if member == nil {
		return d.skipValue()
	}
	value, ok := d.fieldValue(target, member)
	if !ok {
		return d.skipValue()
	}
	if member.quoted {
		return d.quoted(value)
	}
	return d.decode(value)
}

// fieldValue walks to a field, allocating embedded pointers on the way. A nil
// pointer to an unexported embedded struct cannot be set: go.dev/issue/21357.
func (d *Decoder) fieldValue(target reflect.Value, member *field) (reflect.Value, bool) {
	for _, i := range member.index {
		if target.Kind() == reflect.Pointer {
			if target.IsNil() {
				if !target.CanSet() {
					d.keep(errors.New("jrpc: cannot set embedded pointer to unexported struct: " + target.Type().Elem().String()))
					return reflect.Value{}, false
				}
				target.Set(reflect.New(target.Type().Elem()))
			}
			target = target.Elem()
		}
		target = target.Field(i)
	}
	return target, true
}

// quoted decodes a ,string field. encoding/json offers those rules only through
// a struct tag, and they are too intricate to copy, so one is built to reach them.
func (d *Decoder) quoted(value reflect.Value) error {
	raw, err := d.rawValue()
	if err != nil {
		return err
	}
	holder := reflect.New(reflect.StructOf([]reflect.StructField{
		{Name: "V", Type: value.Type(), Tag: `json:",string"`},
	})).Elem()
	holder.Field(0).Set(value)
	object := append(append([]byte(`{"V":`), raw...), '}')
	d.keep(json.Unmarshal(object, holder.Addr().Interface()))
	value.Set(holder.Field(0))
	return nil
}

func (d *Decoder) mapEntry(target reflect.Value, raw []byte, key string) error {
	elem := reflect.New(target.Type().Elem()).Elem()
	if err := d.decode(elem); err != nil {
		return err
	}
	if name, ok := d.mapKey(target.Type().Key(), raw, key); ok {
		target.SetMapIndex(name, elem)
	}
	return nil
}

// mapKey parses a key back into the key type. A key that does not fit is
// dropped with an error kept, leaving the rest of the object to decode.
func (d *Decoder) mapKey(rtype reflect.Type, raw []byte, key string) (reflect.Value, bool) {
	name := reflect.New(rtype).Elem()
	if reflect.PointerTo(rtype).Implements(reflect.TypeFor[encoding.TextUnmarshaler]()) {
		if err := json.Unmarshal(raw, name.Addr().Interface()); err != nil {
			d.keep(err)
			return reflect.Value{}, false
		}
		// The method can fill the key with anything, and hashing a slice panics.
		if !name.Comparable() {
			d.keep(&json.UnmarshalTypeError{Value: "object key " + key, Type: rtype})
			return reflect.Value{}, false
		}
		return name, true
	}
	switch rtype.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		number, err := strconv.ParseInt(key, 10, 64)
		if err != nil || rtype.OverflowInt(number) {
			d.keep(&json.UnmarshalTypeError{Value: "number " + key, Type: rtype})
			return reflect.Value{}, false
		}
		name.SetInt(number)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		number, err := strconv.ParseUint(key, 10, 64)
		if err != nil || rtype.OverflowUint(number) {
			d.keep(&json.UnmarshalTypeError{Value: "number " + key, Type: rtype})
			return reflect.Value{}, false
		}
		name.SetUint(number)
	default:
		name.SetString(key)
	}
	return name, true
}

// array decodes the array at the head of the stream. Like encoding/json it
// reuses existing elements, drops what a fixed array cannot hold, and zeroes
// or truncates what the input did not reach.
func (d *Decoder) array(target reflect.Value) error {
	d.head++
	d.depth++
	if d.depth > maxDepth {
		return exceededMaxDepth()
	}
	count, err := d.elements(target)
	if err != nil {
		return err
	}
	d.depth--
	if target.Kind() == reflect.Slice && count == 0 {
		target.Set(reflect.MakeSlice(target.Type(), 0, 0))
		return nil
	}
	if target.Kind() == reflect.Slice {
		target.SetLen(count)
		return nil
	}
	for i := count; i < target.Len(); i++ {
		target.Index(i).SetZero()
	}
	return nil
}

// elements decodes up to the closing bracket and returns how many it saw.
func (d *Decoder) elements(target reflect.Value) (int, error) {
	if err := d.skipWhitespace(); err != nil {
		return 0, err
	}
	char, err := d.peek_byte()
	if err != nil {
		return 0, err
	}
	if char == ']' {
		d.head++
		return 0, nil
	}
	for index := 0; ; index++ {
		if target.Kind() == reflect.Slice {
			if index >= target.Cap() {
				target.Grow(1)
			}
			if index >= target.Len() {
				target.SetLen(index + 1)
			}
		}
		if err := d.element(target, index); err != nil {
			return 0, err
		}
		if err := d.skipWhitespace(); err != nil {
			return 0, err
		}
		char, err := d.scan_byte()
		if err != nil {
			return 0, err
		}
		if char == ']' {
			return index + 1, nil
		}
		if char != ',' {
			return 0, errors.New("expected comma or closing bracket after array element")
		}
	}
}

// element drops what lies beyond the end of a fixed array.
func (d *Decoder) element(target reflect.Value, index int) error {
	if index < target.Len() {
		return d.decode(target.Index(index))
	}
	return d.skipValue()
}

func (d *Decoder) skipValue() error {
	_, err := d.rawValue()
	return err
}

// rawValue consumes one JSON value and returns its text, leaving the parent's
// comma or closing delimiter for the caller. The text is always well formed.
func (d *Decoder) rawValue() ([]byte, error) {
	if err := d.skipWhitespace(); err != nil {
		return nil, err
	}
	char, err := d.peek_byte()
	if err != nil {
		return nil, err
	}
	var raw []byte
	switch {
	case char == 't':
		raw, err = d.rawLiteral("true")
	case char == 'f':
		raw, err = d.rawLiteral("false")
	case char == 'n':
		raw, err = d.rawLiteral("null")
	case char == '"' || char == '{' || char == '[':
		raw, err = d.rawDelimited()
	case char == '-' || isDigit(char):
		raw, err = d.rawNumber()
	default:
		return nil, errors.New("invalid JSON value")
	}
	if err != nil {
		return nil, err
	}
	if !json.Valid(raw) {
		return nil, errors.New("invalid JSON value")
	}
	return raw, nil
}

// rawLiteral stops at the last letter, so a value at the end of what a socket
// has delivered so far does not wait for a byte that may never come.
func (d *Decoder) rawLiteral(literal string) ([]byte, error) {
	for i := range len(literal) {
		char, err := d.scan_byte()
		if err != nil {
			return nil, err
		}
		if char != literal[i] {
			return nil, errors.New("invalid JSON literal")
		}
	}
	return []byte(literal), nil
}

// rawNumber is the one value with no closing delimiter: only the next byte or
// the end of the stream ends it, and there the number may be complete or cut off.
func (d *Decoder) rawNumber() ([]byte, error) {
	var raw []byte
	for {
		char, err := d.peek_byte()
		if err == io.EOF && json.Valid(raw) {
			return raw, nil
		}
		if err != nil {
			return nil, err
		}
		if !isNumberByte(char) {
			return raw, nil
		}
		raw = append(raw, char)
		d.head++
	}
}

func isNumberByte(char byte) bool {
	return isDigit(char) || char == '-' || char == '+' || char == '.' || char == 'e' || char == 'E'
}

// rawDelimited reads a string, object or array up to its closing delimiter.
// It only tracks where that is; json.Valid judges the text afterwards.
func (d *Decoder) rawDelimited() ([]byte, error) {
	var raw []byte
	var depth int
	var quoted, escaped bool
	for {
		char, err := d.scan_byte()
		if err != nil {
			return nil, err
		}
		raw = append(raw, char)
		switch {
		case quoted && escaped:
			escaped = false
		case quoted:
			escaped = char == '\\'
			quoted = char != '"'
		case char == '"':
			quoted = true
		case char == '[' || char == '{':
			depth++
			if d.depth+depth > maxDepth {
				return nil, exceededMaxDepth()
			}
		case char == ']' || char == '}':
			depth--
		}
		if !quoted && depth == 0 {
			return raw, nil
		}
	}
}
