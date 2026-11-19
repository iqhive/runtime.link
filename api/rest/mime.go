package rest

import (
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

	"runtime.link/api/xray"
)

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
	if err := m.encode(m.k, any); err != nil {
		return xray.New(err)
	}
	return m.w.Close()
}

func (m multipartEncoder) encode(name string, value any) error {
	var (
		rvalue = reflect.ValueOf(value)
	)
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
			if hiddenField(field) || !field.IsExported() {
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
			if err := m.encode(prefix+fname, rvalue.Field(i).Interface()); err != nil {
				return xray.New(err)
			}
		}
	case reflect.Map:
		for _, key := range rvalue.MapKeys() {
			if err := m.encode(fmt.Sprintf("%v", key.Interface()), rvalue.MapIndex(key).Interface()); err != nil {
				return xray.New(err)
			}
		}
	case reflect.Pointer:
		return m.encode(name, rvalue.Elem().Interface())
	default:
		return m.w.WriteField(name, fmt.Sprint(value))
	}
	return nil
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
			v := val
			rtype := reflect.TypeOf(v)
			if rtype.Kind() == reflect.Struct && rtype.NumField() > 0 && rtype.Field(0).Type.Kind() == reflect.Slice {
				v = reflect.ValueOf(v).Field(0).Interface()
			}
			if mapping, ok := v.(map[string]any); ok {
				keys := make([]string, 0, len(mapping))
				for key := range mapping {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					value := mapping[key]
					if reflect.TypeOf(value).Kind() == reflect.Slice {
						v = value
					}
				}
			}
			rtype = reflect.TypeOf(v)
			if enc, ok := v.(encoding.TextMarshaler); ok {
				text, err := enc.MarshalText()
				if err != nil {
					return xray.New(err)
				}
				w.Header().Set("Content-Length", strconv.Itoa(len(text)))
				_, err = w.Write(text)
				return xray.New(err)
			}
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
			_, err = w.Write(text)
			return xray.New(err)
		},
		Decode: func(r io.Reader, v any) error {
			if dec, ok := v.(encoding.TextUnmarshaler); ok {
				var text []byte
				if _, err := io.ReadFull(r, text); err != nil {
					return xray.New(err)
				}
				return xray.New(dec.UnmarshalText(text))
			}
			_, err := fmt.Fscan(r, v)
			return xray.New(err)
		},
	},
	"multipart/form-data": {
		Encode: func(w http.ResponseWriter, v any) error {
			return newMultipartEncoder(w).Encode(v)
		},
	},
	"application/octet-stream": {
		Encode: func(w http.ResponseWriter, v any) error {
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
