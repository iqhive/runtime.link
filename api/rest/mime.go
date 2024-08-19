package rest

import (
	"encoding"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

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
		if file, ok := reader.(*os.File); ok {
			var header = make(textproto.MIMEHeader)
			header.Set("Content-Disposition",
				fmt.Sprintf(`form-data; name=%s; filename=%s`,
					strconv.Quote(name), strconv.Quote(filepath.Base(file.Name()))))
			buffer := make([]byte, 512)
			_, err := file.Read(buffer)
			if err != nil {
				return err
			}
			file.Seek(0, 0)
			header.Set("Content-Type", http.DetectContentType(buffer))
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
		for i := 0; i < rvalue.NumField(); i++ {
			field := rvalue.Type().Field(i)
			prefix := name
			if prefix != "" {
				prefix += "."
			}
			fname := field.Name
			if tag := field.Tag.Get("json"); tag != "" {
				fname = tag
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
			b, err := json.Marshal(v)
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
			b, err := xml.Marshal(v)
			if err != nil {
				return xray.New(err)
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(b)))
			_, err = w.Write(b)
			return xray.New(err)
		},
		Decode: func(r io.Reader, v any) error {
			return xray.New(xml.NewDecoder(r).Decode(v))
		},
	},
	"text/plain": {
		Encode: func(w http.ResponseWriter, v any) error {
			if enc, ok := v.(encoding.TextMarshaler); ok {
				text, err := enc.MarshalText()
				if err != nil {
					return xray.New(err)
				}
				w.Header().Set("Content-Length", strconv.Itoa(len(text)))
				_, err = w.Write(text)
				return xray.New(err)
			}
			_, err := fmt.Fprint(w, v)
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
	"application/json+schema": {
		Encode: func(w http.ResponseWriter, v any) error {
			if err := json.NewEncoder(w).Encode(schemaFor(nil, v)); err != nil {
				return err
			}
			return nil
		},
	},
}
