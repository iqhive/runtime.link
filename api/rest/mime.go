package rest

import (
	"bytes"
	"encoding"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/iqhive/runtime.link/api/xray"
)

// maxValueDepth bounds how deep an encoder follows a value. A cycle has no
// bottom, and running out of stack cannot be recovered from.
const maxValueDepth = 512

var errValueTooDeep = errors.New("value is nested too deeply, or holds a cycle")

// multipartEncoder aims to enable the representation of form-based APIs.
type multipartEncoder struct {
	k string
	w *multipart.Writer
}

func newMultipartEncoder(w http.ResponseWriter) multipartEncoder {
	writer := multipart.NewWriter(w)
	w.Header().Set("Content-Type", writer.FormDataContentType())
	return multipartEncoder{
		w: writer,
	}
}

func (m multipartEncoder) Encode(any any) error {
	if err := m.encode(m.k, any, 0); err != nil {
		return xray.New(err)
	}
	return m.w.Close()
}

func (m multipartEncoder) encode(name string, value any, depth int) error {
	var (
		rvalue = reflect.ValueOf(value)
	)
	if depth > maxValueDepth {
		return errValueTooDeep
	}
	if isNilResult(rvalue) {
		return nil
	}
	if textMarshaler, ok := value.(encoding.TextMarshaler); ok {
		text, err := textMarshaler.MarshalText()
		if err != nil {
			return xray.New(err)
		}
		return m.w.WriteField(name, string(text))
	}
	// TODO/FIXME: there should be a clearly documented way to represent the
	// Content-Disposition and Content-Type for an io.Reader.
	if reader, ok := value.(io.Reader); ok {
		if file, ok := reader.(interface {
			io.Reader
			Name() string
		}); ok {
			var header = make(textproto.MIMEHeader)
			header.Set("Content-Disposition",
				fmt.Sprintf(`form-data; name=%s; filename=%s`,
					strconv.Quote(name), strconv.Quote(file.Name())))
			seeker, ok := reader.(io.Seeker)
			if ok {
				buffer := make([]byte, 512)
				_, err := file.Read(buffer)
				if err != nil {
					return err
				}
				header.Set("Content-Type", http.DetectContentType(buffer))
				seeker.Seek(0, 0)
			}
			w, err := m.w.CreatePart(header)
			if err != nil {
				return xray.New(err)
			}
			if _, err := io.Copy(w, reader); err != nil {
				return xray.New(err)
			}
			return nil
		}
		w, err := m.w.CreateFormField(name)
		if err != nil {
			return xray.New(err)
		}
		if _, err := io.Copy(w, reader); err != nil {
			return xray.New(err)
		}
		return nil
	}
	switch rvalue.Kind() {
	case reflect.Struct:
		for i := range rvalue.NumField() {
			field := rvalue.Type().Field(i)
<<<<<<< HEAD
			if !field.IsExported() || hiddenField(field) {
=======
			if hiddenField(field) || !field.IsExported() {
>>>>>>> upstream-translated
				continue
			}
			prefix := name
			if prefix != "" {
				prefix += "."
			}
			if name == "_" {
				prefix = ""
			}
			fname := field.Name
			if tag := field.Tag.Get("json"); tag != "" {
				name, flags, _ := strings.Cut(tag, ",")
				if name != "" {
					fname = name
				}
				if strings.Contains(flags, "omitempty") || strings.Contains(flags, "omitzero") {
					if rvalue.Field(i).IsZero() {
						continue
					}
				}
			}
			if tag := field.Tag.Get("form"); tag != "" {
				fname = tag
			}
			if err := m.encode(prefix+fname, rvalue.Field(i).Interface(), depth+1); err != nil {
				return xray.New(err)
			}
		}
	case reflect.Map:
		for _, key := range rvalue.MapKeys() {
			if err := m.encode(keyText(key), rvalue.MapIndex(key).Interface(), depth+1); err != nil {
				return xray.New(err)
			}
		}
	case reflect.Pointer:
		return m.encode(name, rvalue.Elem().Interface(), depth+1)
	case reflect.Slice, reflect.Array:
		if isByteSlice(rvalue.Type()) {
			return m.w.WriteField(name, string(rvalue.Bytes()))
		}
		for i := range rvalue.Len() {
			if err := m.encode(name, rvalue.Index(i).Interface(), depth+1); err != nil {
				return xray.New(err)
			}
		}
	default:
		return m.w.WriteField(name, fmt.Sprint(value))
	}
	return nil
}

// withoutHiddenFields copies v with its hidden fields zeroed, for an encoder
// that only knows its own struct tags.
func withoutHiddenFields(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	src := reflect.ValueOf(v)
	dst := reflect.New(src.Type()).Elem()
	if err := copyVisible(dst, src, 0); err != nil {
		return nil, err
	}
	return dst.Interface(), nil
}

// clearHiddenFields zeroes the hidden fields of what v points to, for a decoder
// that only knows its own struct tags. A hidden field cannot be told apart from
// the rest while decoding, so it is removed once that is done.
func clearHiddenFields(v any) error {
	target := reflect.ValueOf(v).Elem()
	visible := reflect.New(target.Type()).Elem()
	if err := copyVisible(visible, target, 0); err != nil {
		return err
	}
	target.Set(visible)
	return nil
}

// copyVisible deep copies src into the zero value dst, leaving out hidden and
// unexported fields. A copy is what keeps the caller's own value, reachable
// through its pointers and maps, intact.
func copyVisible(dst, src reflect.Value, depth int) error {
	if depth > maxValueDepth {
		return errValueTooDeep
	}
	if dst.CanSet() && definesOwnWireForm(src.Type()) {
		dst.Set(src)
		return nil
	}
	switch src.Kind() {
	case reflect.Pointer:
		if src.IsNil() {
			return nil
		}
		dst.Set(reflect.New(src.Type().Elem()))
		return copyVisible(dst.Elem(), src.Elem(), depth+1)
	case reflect.Interface:
		if src.IsNil() {
			return nil
		}
		elem := reflect.New(src.Elem().Type()).Elem()
		if err := copyVisible(elem, src.Elem(), depth+1); err != nil {
			return err
		}
		dst.Set(elem)
	case reflect.Struct:
		for i := range src.NumField() {
			field := src.Type().Field(i)
			if hiddenField(field) || !field.IsExported() && !isEmbeddedStruct(field) {
				continue
			}
			if err := copyVisible(dst.Field(i), src.Field(i), depth+1); err != nil {
				return err
			}
		}
	case reflect.Map:
		if src.IsNil() {
			return nil
		}
		dst.Set(reflect.MakeMapWithSize(src.Type(), src.Len()))
		for entries := src.MapRange(); entries.Next(); {
			value := reflect.New(src.Type().Elem()).Elem()
			if err := copyVisible(value, entries.Value(), depth+1); err != nil {
				return err
			}
			dst.SetMapIndex(entries.Key(), value)
		}
	case reflect.Slice:
		if src.IsNil() {
			return nil
		}
		dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Len()))
		if isScalarKind(src.Type().Elem().Kind()) {
			reflect.Copy(dst, src)
			return nil
		}
		return copyElements(dst, src, depth)
	case reflect.Array:
		return copyElements(dst, src, depth)
	default:
		dst.Set(src)
	}
	return nil
}

func copyElements(dst, src reflect.Value, depth int) error {
	for i := range src.Len() {
		if err := copyVisible(dst.Index(i), src.Index(i), depth+1); err != nil {
			return err
		}
	}
	return nil
}

// isEmbeddedStruct reports whether an unexported field is one whose exported
// fields are promoted into the struct that embeds it.
func isEmbeddedStruct(field reflect.StructField) bool {
	return field.Anonymous && field.Type.Kind() == reflect.Struct
}

// definesOwnWireForm reports whether a type renders itself, with fields that are
// only hidden from those who would walk them. time.Time is one.
func definesOwnWireForm(rtype reflect.Type) bool {
	pointer := reflect.PointerTo(rtype)
	for _, form := range []reflect.Type{
		reflect.TypeFor[encoding.TextMarshaler](),
		reflect.TypeFor[encoding.TextUnmarshaler](),
		reflect.TypeFor[xml.Marshaler](),
		reflect.TypeFor[xml.Unmarshaler](),
	} {
		if rtype.Implements(form) || pointer.Implements(form) {
			return true
		}
	}
	return false
}

// plainText renders a value for text/plain: a list one element to a line. A
// []byte is its bytes, and a slice that reads itself with UnmarshalJSON is one
// value, as each is read back as one. Only values with a text form are printed:
// what has none, a struct above all, goes through JSON and its rules about which
// fields exist, as fmt would print every field there is.
func plainText(val any) ([]byte, error) {
	if mapping, ok := val.(map[string]any); ok {
		keys := make([]string, 0, len(mapping))
		for key := range mapping {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if value := reflect.ValueOf(mapping[key]); value.Kind() == reflect.Slice {
				val = mapping[key]
			}
		}
	}
	value := reflect.ValueOf(val)
	for value.Kind() == reflect.Pointer && !value.IsNil() && !printsItself(value) {
		value = value.Elem()
	}
	if listed := listedField(value); listed.IsValid() {
		value = listed
	}
	if value.Kind() != reflect.Slice || isByteSlice(value.Type()) || isTextMarshaler(value) {
		return plainValue(value)
	}
	if readsJSONText(value.Type()) {
		return wholeText(value)
	}
	var lines bytes.Buffer
	for i := range value.Len() {
		line, err := plainValue(value.Index(i))
		if err != nil {
			return nil, err
		}
		lines.Write(line)
		lines.WriteByte('\n')
	}
	return lines.Bytes(), nil
}

// wholeText renders a slice that is one value to whoever reads it, by its own
// UnmarshalJSON, and not a list of them: written a line to an element, it would
// be read back as one value with newlines in it. It is written as a path or a
// query carries it, the JSON it makes of itself with the quotes off a string.
func wholeText(value reflect.Value) ([]byte, error) {
	encoded, err := json.Marshal(value.Interface())
	if err != nil {
		return nil, err
	}
	var text string
	if json.Unmarshal(encoded, &text) == nil {
		return []byte(text), nil
	}
	return encoded, nil
}

// readsJSONText reports whether a slice type is read from text by its own
// UnmarshalJSON, which [scanText] only asks when there is no UnmarshalText.
func readsJSONText(rtype reflect.Type) bool {
	pointer := reflect.PointerTo(rtype)
	return pointer.Implements(reflect.TypeFor[json.Unmarshaler]()) && !pointer.Implements(reflect.TypeFor[encoding.TextUnmarshaler]())
}

func isTextMarshaler(value reflect.Value) bool {
	_, ok := value.Interface().(encoding.TextMarshaler)
	return ok
}

// printsItself reports whether a value has a text form of its own, which is the
// one to print, through a pointer receiver as much as any.
func printsItself(value reflect.Value) bool {
	switch value.Interface().(type) {
	case encoding.TextMarshaler, error, fmt.Stringer:
		return true
	}
	return false
}

// listedField returns the slice a struct wraps, when it is the first field, as
// such a struct is a list with some metadata the text form has no room for.
func listedField(value reflect.Value) reflect.Value {
	if value.Kind() != reflect.Struct || value.NumField() == 0 {
		return reflect.Value{}
	}
	field := value.Type().Field(0)
	if field.Type.Kind() != reflect.Slice || !field.IsExported() || hiddenField(field) {
		return reflect.Value{}
	}
	return value.Field(0)
}

// plainValue renders one value for text/plain, in its own text form, else as fmt
// prints it where that is a value and not a dump of fields.
func plainValue(value reflect.Value) ([]byte, error) {
	if !value.IsValid() {
		return nil, nil
	}
	for {
		if isNilResult(value) {
			return []byte("<nil>"), nil
		}
		switch text := value.Interface().(type) {
		case encoding.TextMarshaler:
			return text.MarshalText()
		case error:
			return []byte(text.Error()), nil
		case fmt.Stringer:
			return []byte(text.String()), nil
		}
		if value.Kind() != reflect.Pointer && value.Kind() != reflect.Interface {
			break
		}
		value = value.Elem()
	}
	if isScalarKind(value.Kind()) {
		return fmt.Append(nil, value.Interface()), nil
	}
	if isByteSlice(value.Type()) {
		return value.Bytes(), nil
	}
	return json.Marshal(value.Interface())
}

// hasPlainForm reports whether text/plain shows a value as a value: one with a
// text form of its own, a scalar, nil, or a list of those.
func hasPlainForm(value reflect.Value) bool {
	for {
		if isNilResult(value) || printsItself(value) {
			return true
		}
		if value.Kind() != reflect.Pointer && value.Kind() != reflect.Interface {
			break
		}
		value = value.Elem()
	}
	if value.Kind() == reflect.Slice {
		return hasPlainElements(value.Type().Elem())
	}
	return isScalarKind(value.Kind())
}

func hasPlainElements(elem reflect.Type) bool {
	for _, form := range []reflect.Type{
		reflect.TypeFor[encoding.TextMarshaler](),
		reflect.TypeFor[error](),
		reflect.TypeFor[fmt.Stringer](),
	} {
		if elem.Implements(form) {
			return true
		}
	}
	for elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	return isScalarKind(elem.Kind())
}

// xmlElement is a value for application/xml that has no element of its own to
// be written as. encoding/xml names an element after its type, and the struct
// that holds what travels under ARGUMENT_RULES or RESULT_RULES has no name.
type xmlElement struct {
	name  string
	value any
}

// mappedElement is the arguments or results of a function as the fields of its
// mapping type, each an element named by its rule, inside one named after the
// function. A map of them is what the other encoders are given, but XML has no
// way to write a map.
func mappedElement(name string, mappingType reflect.Type, values []reflect.Value) xmlElement {
	mapped := reflect.New(mappingType).Elem()
	for i, value := range values {
		mapped.Field(i).Set(value)
	}
	return xmlElement{name: name, value: mapped.Interface()}
}

// marshalXML writes v without its hidden fields, which XML has no tag to leave
// out by.
func marshalXML(v any) ([]byte, error) {
	element, named := v.(xmlElement)
	if named {
		v = element.value
	}
	visible, err := withoutHiddenFields(v)
	if err != nil {
		return nil, err
	}
	if !named {
		return xml.Marshal(visible)
	}
	var encoded bytes.Buffer
	start := xml.StartElement{Name: xml.Name{Local: element.name}}
	if err := xml.NewEncoder(&encoded).EncodeElement(visible, start); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

// xmlWrites reports whether encoding/xml has a way to write a value of a type,
// so that a client is not offered application/xml for a result it does not.
// It refuses what has no text and no fields, a map above all, wherever in a
// value it is, and at the top what has no name to call its element by, unless
// isNamed says the element is named for it, as that of a field is. An interface
// may hold anything, and is taken to hold what can be written. seen is the
// types that have been looked at: a type may hold itself, which is no further
// reason that it cannot be written, and has no end to walk to.
func xmlWrites(rtype reflect.Type, isNamed bool, seen map[reflect.Type]bool) bool {
	var pointers []reflect.Type
	for rtype.Kind() == reflect.Pointer {
		if slices.Contains(pointers, rtype) {
			// A pointer to nothing but itself, which encoding/xml follows
			// without ever coming to the end of it.
			return false
		}
		pointers = append(pointers, rtype)
		rtype = rtype.Elem()
	}
	if seen[rtype] {
		return true
	}
	seen[rtype] = true
	if writesOwnXML(rtype) {
		return true
	}
	isList := (rtype.Kind() == reflect.Slice || rtype.Kind() == reflect.Array) && rtype.Elem().Kind() != reflect.Uint8
	if isList {
		// Each element is an element of its own, which the name of the list's
		// type is no name for.
		return xmlWrites(rtype.Elem(), isNamed, seen)
	}
	isNamed = isNamed || rtype.Name() != ""
	switch rtype.Kind() {
	case reflect.Interface:
		return true
	case reflect.Slice, reflect.Array:
		return isNamed
	case reflect.Struct:
		return xmlWritesFields(rtype, isNamed, seen)
	case reflect.Bool, reflect.String, reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return isNamed
	}
	return false
}

// xmlWritesFields is [xmlWrites] for a struct, which is written if each of its
// fields is. A field that XML leaves out is not asked, and nor is one left out
// when it is empty, as the value may be one that can be written.
func xmlWritesFields(rtype reflect.Type, isNamed bool, seen map[reflect.Type]bool) bool {
	for i := range rtype.NumField() {
		field := rtype.Field(i)
		tag := field.Tag.Get("xml")
		if field.Name == "XMLName" {
			isNamed = true
			continue
		}
		_, options, _ := strings.Cut(tag, ",")
		isLeftOut := tag == "-" || !field.IsExported() && !field.Anonymous
		if isLeftOut || slices.Contains(strings.Split(options, ","), "omitempty") {
			continue
		}
		if !xmlWrites(field.Type, true, seen) {
			return false
		}
	}
	return isNamed
}

// writesOwnXML reports whether a type has a method that encoding/xml writes it
// with, on either receiver, as most values are reached through a pointer.
func writesOwnXML(rtype reflect.Type) bool {
	pointer := reflect.PointerTo(rtype)
	for _, form := range []reflect.Type{reflect.TypeFor[xml.Marshaler](), reflect.TypeFor[encoding.TextMarshaler]()} {
		if rtype.Implements(form) || pointer.Implements(form) {
			return true
		}
	}
	return false
}

type contentType struct {
	Encode func(w http.ResponseWriter, v any) error
	Decode func(r io.Reader, v any) error
}

var contentTypes = map[string]contentType{
	"application/json": {
		Encode: func(w http.ResponseWriter, v any) error {
			b, err := json.MarshalIndent(v, "", "\t")
			if err != nil {
				return xray.New(err)
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(b)))
			_, err = w.Write(b)
			return xray.New(err)
		},
		Decode: func(r io.Reader, v any) error {
			return xray.New(json.NewDecoder(r).Decode(v))
		},
	},
	"application/xml": {
		Encode: func(w http.ResponseWriter, v any) error {
<<<<<<< HEAD
			b, err := marshalXML(v)
=======
			visible, err := withoutHiddenFields(v)
			if err != nil {
				return xray.New(err)
			}
			b, err := xml.Marshal(visible)
>>>>>>> upstream-translated
			if err != nil {
				return xray.New(err)
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(b)))
			_, err = w.Write(b)
			return xray.New(err)
		},
		Decode: func(r io.Reader, v any) error {
			if err := xml.NewDecoder(r).Decode(v); err != nil {
				return xray.New(err)
			}
			return xray.New(clearHiddenFields(v))
		},
	},
	"text/plain": {
		Encode: func(w http.ResponseWriter, val any) error {
			text, err := plainText(val)
			if err != nil {
				return xray.New(err)
			}
<<<<<<< HEAD
			w.Header().Set("Content-Length", strconv.Itoa(len(text)))
=======
			if rtype.Kind() == reflect.Slice {
				rvalue := reflect.ValueOf(v)
				for i := 0; i < rvalue.Len(); i++ {
					if enc, ok := v.(encoding.TextMarshaler); ok {
						text, err := enc.MarshalText()
						if err != nil {
							return xray.New(err)
						}
						_, err = fmt.Fprint(w, text)
						if err != nil {
							return xray.New(err)
						}
					} else {
						text, err := plainValue(rvalue.Index(i).Interface())
						if err != nil {
							return xray.New(err)
						}
						if _, err := w.Write(text); err != nil {
							return xray.New(err)
						}
					}
					_, err := fmt.Fprintln(w)
					if err != nil {
						return xray.New(err)
					}
				}
				return nil
			}
			text, err := plainValue(v)
			if err != nil {
				return xray.New(err)
			}
>>>>>>> upstream-translated
			_, err = w.Write(text)
			return xray.New(err)
		},
		Decode: func(r io.Reader, v any) error {
			text, err := io.ReadAll(r)
			if err != nil {
				return xray.New(err)
			}
			return xray.New(scanBodyText(v, string(text)))
		},
	},
	"multipart/form-data": {
		Encode: func(w http.ResponseWriter, v any) error {
			return newMultipartEncoder(w).Encode(v)
		},
	},
	"application/octet-stream": {
		Encode: func(w http.ResponseWriter, v any) error {
			if isNilResult(reflect.ValueOf(v)) {
				return xray.New(fmt.Errorf("cannot encode nil %T as application/octet-stream", v))
			}
			closer, ok := v.(io.Closer)
			if ok && closer != nil {
				defer closer.Close()
			}
			if reader, ok := v.(io.Reader); ok {
				_, err := io.Copy(w, reader)
				return xray.New(err)
			}
			return xray.New(fmt.Errorf("cannot encode type %T as application/octet-stream", v))
		},
		Decode: func(r io.Reader, v any) error {
			if writer, ok := v.(io.Writer); ok {
				_, err := io.Copy(writer, r)
				return xray.New(err)
			}
			return xray.New(fmt.Errorf("cannot decode type %T as application/octet-stream", v))
		},
	},
	"application/json+schema": {
		Encode: func(w http.ResponseWriter, v any) error {
			if err := json.NewEncoder(w).Encode(schemaFor(nil, v)); err != nil {
				return err
			}
			return nil
		},
	},
}

// maxValueDepth bounds how deep a value is followed while copying it, so that a
// cyclic value is an error rather than a fatal stack overflow.
const maxValueDepth = 512

var errValueTooDeep = errors.New("value is nested too deeply, or refers to itself")

// hiddenField reports whether a field is left out of the API, as it is from
// JSON.
func hiddenField(field reflect.StructField) bool {
	return field.Tag.Get("json") == "-"
}

// plainValue renders a single value for text/plain. Only values with a text
// form are printed with fmt: anything else, a struct above all, goes through
// JSON and its rules about which fields exist, as fmt would print every field,
// hidden and unexported ones included.
func plainValue(v any) ([]byte, error) {
	switch v := v.(type) {
	case nil:
		return nil, nil
	case encoding.TextMarshaler:
		return v.MarshalText()
	case error:
		return []byte(v.Error()), nil
	case fmt.Stringer:
		return []byte(v.String()), nil
	case []byte:
		return v, nil
	}
	rvalue := reflect.ValueOf(v)
	for rvalue.Kind() == reflect.Pointer || rvalue.Kind() == reflect.Interface {
		if rvalue.IsNil() {
			return nil, nil
		}
		rvalue = rvalue.Elem()
	}
	switch rvalue.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.String:
		return []byte(fmt.Sprint(rvalue.Interface())), nil
	}
	return json.Marshal(v)
}

// withoutHiddenFields copies v with its hidden and unexported fields zeroed, for
// an encoder that only knows its own struct tags.
func withoutHiddenFields(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	src := reflect.ValueOf(v)
	dst := reflect.New(src.Type()).Elem()
	if err := copyVisible(dst, src, 0); err != nil {
		return nil, err
	}
	return dst.Interface(), nil
}

// clearHiddenFields zeroes the hidden fields of what v points to, for a decoder
// that only knows its own struct tags, so that a client cannot set them.
func clearHiddenFields(v any) error {
	rvalue := reflect.ValueOf(v)
	if rvalue.Kind() != reflect.Pointer || rvalue.IsNil() {
		return nil
	}
	target := rvalue.Elem()
	visible := reflect.New(target.Type()).Elem()
	if err := copyVisible(visible, target, 0); err != nil {
		return err
	}
	target.Set(visible)
	return nil
}

// copyVisible deep copies src into the zero value dst, leaving out hidden and
// unexported fields. A copy keeps the caller's own value, reachable through its
// pointers and maps, intact.
func copyVisible(dst, src reflect.Value, depth int) error {
	if depth > maxValueDepth {
		return errValueTooDeep
	}
	if definesOwnWireForm(src.Type()) {
		dst.Set(src)
		return nil
	}
	switch src.Kind() {
	case reflect.Pointer:
		if src.IsNil() {
			return nil
		}
		dst.Set(reflect.New(src.Type().Elem()))
		return copyVisible(dst.Elem(), src.Elem(), depth+1)
	case reflect.Interface:
		if src.IsNil() {
			return nil
		}
		elem := reflect.New(src.Elem().Type()).Elem()
		if err := copyVisible(elem, src.Elem(), depth+1); err != nil {
			return err
		}
		dst.Set(elem)
	case reflect.Struct:
		for i := range src.NumField() {
			field := src.Type().Field(i)
			if hiddenField(field) || !field.IsExported() {
				continue
			}
			if err := copyVisible(dst.Field(i), src.Field(i), depth+1); err != nil {
				return err
			}
		}
	case reflect.Map:
		if src.IsNil() {
			return nil
		}
		dst.Set(reflect.MakeMapWithSize(src.Type(), src.Len()))
		for entries := src.MapRange(); entries.Next(); {
			value := reflect.New(src.Type().Elem()).Elem()
			if err := copyVisible(value, entries.Value(), depth+1); err != nil {
				return err
			}
			dst.SetMapIndex(entries.Key(), value)
		}
	case reflect.Slice:
		if src.IsNil() {
			return nil
		}
		dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Len()))
		fallthrough
	case reflect.Array:
		for i := range src.Len() {
			if err := copyVisible(dst.Index(i), src.Index(i), depth+1); err != nil {
				return err
			}
		}
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		// not encodable, left zero.
	default:
		dst.Set(src)
	}
	return nil
}

// definesOwnWireForm reports whether a type renders itself, with fields that are
// only hidden from those who would walk them. time.Time is one.
func definesOwnWireForm(rtype reflect.Type) bool {
	pointer := reflect.PointerTo(rtype)
	for _, form := range []reflect.Type{
		reflect.TypeFor[encoding.TextMarshaler](),
		reflect.TypeFor[encoding.TextUnmarshaler](),
		reflect.TypeFor[xml.Marshaler](),
		reflect.TypeFor[xml.Unmarshaler](),
	} {
		if rtype.Implements(form) || pointer.Implements(form) {
			return true
		}
	}
	return false
}
