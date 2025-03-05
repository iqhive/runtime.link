package jrpc

import (
	"io"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"

	_ "unsafe"
)

type Encoder struct {
	w io.Writer
}

//go:linkname write runtime.link/api/jrpc.write_impl
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
	if err := e.encode(reflect.ValueOf(v)); err != nil {
		return err
	}
	return nil
}

func (e *Encoder) encode(rvalue reflect.Value) error {
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
	case reflect.Uint, reflect.Uint64:
		var raw [20]byte
		buf := strconv.AppendUint(raw[:0:len(raw)], rvalue.Uint(), 10)
		if _, err := write(e.w, buf); err != nil {
			return err
		}
	case reflect.Float32, reflect.Float64:
		var raw [20]byte
		buf := strconv.AppendFloat(raw[:0:len(raw)], rvalue.Float(), 'f', -1, 64)
		if _, err := write(e.w, buf); err != nil {
			return err
		}
	case reflect.String:
		if err := e.string(rvalue.String()); err != nil {
			return err
		}
	case reflect.Array, reflect.Slice:
		if rvalue.IsNil() {
			if _, err := write(e.w, []byte("null")); err != nil {
				return err
			}
			return nil
		}
		if rtype.Elem().Kind() == reflect.Uint8 {
			return e.base64(rvalue.Bytes())
		}
		if _, err := write(e.w, []byte{'['}); err != nil {
			return err
		}
		for i := range rvalue.Len() {
			if err := e.encode(rvalue.Index(i)); err != nil {
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
		if rvalue.IsNil() {
			if _, err := write(e.w, []byte("null")); err != nil {
				return err
			}
			return nil
		}
		return encode_map_noescape(e, rvalue)
	case reflect.Interface, reflect.Pointer:
		if rvalue.IsNil() {
			if _, err := write(e.w, []byte("null")); err != nil {
				return err
			}
			return nil
		}
		return e.encode(rvalue.Elem())
	case reflect.Struct:
		if _, err := write(e.w, []byte{'{'}); err != nil {
			return err
		}
		for i := range rvalue.NumField() {
			field := rtype.Field(i)
			name := field.Name
			var omitEmpty bool
			var omitZero bool
			if field.Tag != "" {
				jtag := field.Tag.Get("json")
				rename, _, hasOpts := strings.Cut(jtag, ",")
				if rename != "" {
					name = rename
				}
				if hasOpts {
					omitEmpty = strings.Contains(jtag, ",omitempty")
					omitZero = strings.Contains(jtag, ",omitzero")
				}
			}
			if omitZero || (omitEmpty && field.Type.Kind() != reflect.Struct && (field.Type.Kind() != reflect.Array || field.Type.Len() == 0)) && rvalue.IsZero() {
				continue
			}
			if _, err := write(e.w, []byte{'"'}); err != nil {
				return err
			}
			if _, err := write(e.w, []byte(name)); err != nil {
				return err
			}
			if _, err := write_string(e.w, `":`); err != nil {
				return err
			}
			if err := e.encode(rvalue.Field(i)); err != nil {
				return err
			}
		}
		if _, err := write(e.w, []byte{'}'}); err != nil {
			return err
		}
	}
	return nil
}

//go:linkname encode_map_noescape runtime.link/api/jrpc.encode_map
//go:noescape
func encode_map_noescape(e *Encoder, rvalue reflect.Value) error

//go:linkname reflect_new_at_noescape runtime.link/api/jrpc.reflect_new_at
//go:noescape
func reflect_new_at_noescape(t reflect.Type, p unsafe.Pointer) reflect.Value

func reflect_new_at(t reflect.Type, p unsafe.Pointer) reflect.Value {
	return reflect.NewAt(t, p)
}

func encode_map(e *Encoder, rvalue reflect.Value) error {
	if _, err := write(e.w, []byte{'{'}); err != nil {
		return err
	}
	iter := rvalue.MapRange()
	first := true
	var keyBuf, valBuf [3]uintptr
	var key, val reflect.Value
	if rvalue.Type().Key().Size() < 24 {
		key = reflect_new_at_noescape(rvalue.Type().Key(), unsafe.Pointer(&keyBuf)).Elem()
	} else {
		key = reflect.New(rvalue.Type().Key()).Elem()
	}
	if rvalue.Type().Elem().Size() < 24 {
		val = reflect_new_at_noescape(rvalue.Type().Elem(), unsafe.Pointer(&valBuf)).Elem()
	} else {
		val = reflect.New(rvalue.Type().Elem()).Elem()
	}
	for iter.Next() {
		if !first {
			if _, err := write(e.w, []byte{','}); err != nil {
				return err
			}
		}
		first = false
		key.SetIterKey(iter)
		val.SetIterValue(iter)
		if err := e.encode(key); err != nil {
			return err
		}
		if _, err := write(e.w, []byte{':'}); err != nil {
			return err
		}
		if err := e.encode(val); err != nil {
			return err
		}
	}
	if _, err := write(e.w, []byte{'}'}); err != nil {
		return err
	}
	return nil
}

// string is ported from encoding/json
// support has been dropped for htmlEscape.
func (e *Encoder) string(s string) error {
	const hex = "0123456789abcdef"
	write(e.w, []byte{'"'})
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if !(b <= 31 || b == '"' || b == '\\') {
				i++
				continue
			}
			if start < i {
				write_string(e.w, s[start:i])
			}
			write(e.w, []byte{'\\'})
			switch b {
			case '\\', '"':
				write(e.w, []byte{b})
			case '\n':
				write(e.w, []byte{'n'})
			case '\r':
				write(e.w, []byte{'r'})
			case '\t':
				write(e.w, []byte{'t'})
			default:
				// This encodes bytes < 0x20 except for \t, \n and \r.
				// If escapeHTML is set, it also escapes <, >, and &
				// because they can lead to security holes when
				// user-controlled strings are rendered into JSON
				// and served to some browsers.
				write_string(e.w, `u00`)
				write(e.w, []byte{hex[b>>4]})
				write(e.w, []byte{hex[b&0xF]})
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				write_string(e.w, s[start:i])
			}
			write_string(e.w, `\ufffd`)
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
				write_string(e.w, s[start:i])
			}
			write_string(e.w, `\u202`)
			write(e.w, []byte{hex[c&0xF]})
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		write_string(e.w, s[start:])
	}
	write(e.w, []byte{'"'})
	return nil
}

// base64 is an efficient base64 encoder, adapted from encoding/base64.
func (m *Encoder) base64(b []byte) error {
	const std = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	if len(b) == 0 {
		_, err := write_string(m.w, `"null"`)
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
