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
			if !field.IsExported() || hiddenField(field) {
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
			if err := m.encode(fmt.Sprintf("%v", key.Interface()), rvalue.MapIndex(key).Interface(), depth+1); err != nil {
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

// plainText renders a value for text/plain: a list one element to a line, which
// for a []byte is a byte to a line, as it has always been. Only values with a
// text form are printed: what has none, a struct above all, goes through JSON
// and its rules about which fields exist, as fmt would print every field there is.
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
	if value.Kind() != reflect.Slice || isTextMarshaler(value) {
		return plainValue(value)
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
			visible, err := withoutHiddenFields(v)
			if err != nil {
				return xray.New(err)
			}
			b, err := xml.Marshal(visible)
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
			w.Header().Set("Content-Length", strconv.Itoa(len(text)))
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
