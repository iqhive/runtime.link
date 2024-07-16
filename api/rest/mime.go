package rest

import (
	"encoding"
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

func newMultipartEncoder(w io.Writer) multipartEncoder {
	return multipartEncoder{
		w: multipart.NewWriter(w),
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
