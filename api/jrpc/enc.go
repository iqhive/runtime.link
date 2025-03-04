package jrpc

import (
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"

	"runtime.link/bit"

	_ "unsafe"
)

type Encoder struct {
	w bit.Writer
}

func NewEncoder(w bit.Writer) *Encoder {
	return &Encoder{w: w}
}

func (e *Encoder) Encode(v any) error {
	return e.encode(reflect.ValueOf(v))
}

func (e *Encoder) encode(rvalue reflect.Value) error {
	rtype := rvalue.Type()
	switch rtype.Kind() {
	case reflect.Bool:
		if rvalue.Bool() {
			if _, err := e.w.WriteBits(bit.String("true")); err != nil {
				return err
			}
		} else {
			if _, err := e.w.WriteBits(bit.String("false")); err != nil {
				return err
			}
		}
	case reflect.Int32, reflect.Int16, reflect.Int8:
		var raw [11]byte
		buf := strconv.AppendInt(raw[:0:len(raw)], rvalue.Int(), 10)
		if _, err := e.w.WriteBits(bit.Bytes(buf...)); err != nil {
			return err
		}
	case reflect.Int, reflect.Int64:
		var raw [20]byte
		buf := strconv.AppendInt(raw[:0:len(raw)], rvalue.Int(), 10)
		if _, err := e.w.WriteBits(bit.Bytes(buf...)); err != nil {
			return err
		}
	case reflect.Uint32, reflect.Uint16, reflect.Uint8:
		var raw [11]byte
		buf := strconv.AppendUint(raw[:0:len(raw)], rvalue.Uint(), 10)
		if _, err := e.w.WriteBits(bit.Bytes(buf...)); err != nil {
			return err
		}
	case reflect.Uint, reflect.Uint64:
		var raw [20]byte
		buf := strconv.AppendUint(raw[:0:len(raw)], rvalue.Uint(), 10)
		if _, err := e.w.WriteBits(bit.Bytes(buf...)); err != nil {
			return err
		}
	case reflect.Float32, reflect.Float64:
		var raw [20]byte
		buf := strconv.AppendFloat(raw[:0:len(raw)], rvalue.Float(), 'f', -1, 64)
		if _, err := e.w.WriteBits(bit.Bytes(buf...)); err != nil {
			return err
		}
	case reflect.String:
		if err := e.string(rvalue.String()); err != nil {
			return err
		}
	case reflect.Array, reflect.Slice:
		if rvalue.IsNil() {
			if _, err := e.w.WriteBits(bit.String("null")); err != nil {
				return err
			}
			return nil
		}
		if rtype.Elem().Kind() == reflect.Uint8 {
			return e.base64(rvalue.Bytes())
		}
		if _, err := e.w.WriteBits(bit.Bytes('[')); err != nil {
			return err
		}
		for i := range rvalue.Len() {
			if err := e.encode(rvalue.Index(i)); err != nil {
				return err
			}
			if i < rvalue.Len()-1 {
				if _, err := e.w.WriteBits(bit.Bytes(',')); err != nil {
					return err
				}
			}
		}
		if _, err := e.w.WriteBits(bit.Bytes(']')); err != nil {
			return err
		}
	case reflect.Map:
		if rvalue.IsNil() {
			if _, err := e.w.WriteBits(bit.String("null")); err != nil {
				return err
			}
			return nil
		}
		return encode_map_noescape(e, rvalue)
	case reflect.Interface, reflect.Pointer:
		if rvalue.IsNil() {
			if _, err := e.w.WriteBits(bit.String("null")); err != nil {
				return err
			}
			return nil
		}
		return e.encode(rvalue.Elem())
	case reflect.Struct:
		if _, err := e.w.WriteBits(bit.Bytes('{')); err != nil {
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
			if err := e.string(name); err != nil {
				return err
			}
			if _, err := e.w.WriteBits(bit.Bytes(':')); err != nil {
				return err
			}
			if err := e.encode(rvalue.Field(i)); err != nil {
				return err
			}
		}
		if _, err := e.w.WriteBits(bit.Bytes('}')); err != nil {
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
	if _, err := e.w.WriteBits(bit.Bytes('{')); err != nil {
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
			if _, err := e.w.WriteBits(bit.Bytes(',')); err != nil {
				return err
			}
		}
		first = false
		key.SetIterKey(iter)
		val.SetIterValue(iter)
		if err := e.encode(key); err != nil {
			return err
		}
		if _, err := e.w.WriteBits(bit.Bytes(':')); err != nil {
			return err
		}
		if err := e.encode(val); err != nil {
			return err
		}
	}
	if _, err := e.w.WriteBits(bit.Bytes('}')); err != nil {
		return err
	}
	return nil
}

// string is ported from encoding/json
// support has been dropped for htmlEscape.
func (e *Encoder) string(s string) error {
	const hex = "0123456789abcdef"
	if _, err := e.w.WriteBits(bit.Bytes('"')); err != nil {
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
				if _, err := e.w.WriteBits(bit.String(s[start:i])); err != nil {
					return err
				}
			}
			if _, err := e.w.WriteBits(bit.Bytes('\\')); err != nil {
				return err
			}
			switch b {
			case '\\', '"':
				if _, err := e.w.WriteBits(bit.Bytes(b)); err != nil {
					return err
				}
			case '\n':
				if _, err := e.w.WriteBits(bit.Bytes('n')); err != nil {
					return err
				}
			case '\r':
				if _, err := e.w.WriteBits(bit.Bytes('r')); err != nil {
					return err
				}
			case '\t':
				if _, err := e.w.WriteBits(bit.Bytes('t')); err != nil {
					return err
				}
			default:
				// This encodes bytes < 0x20 except for \t, \n and \r.
				// If escapeHTML is set, it also escapes <, >, and &
				// because they can lead to security holes when
				// user-controlled strings are rendered into JSON
				// and served to some browsers.
				if _, err := e.w.WriteBits(bit.String(`u00`)); err != nil {
					return err
				}
				if _, err := e.w.WriteBits(bit.Bytes(hex[b>>4])); err != nil {
					return err
				}
				if _, err := e.w.WriteBits(bit.Bytes(hex[b&0xF])); err != nil {
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
				if _, err := e.w.WriteBits(bit.String(s[start:i])); err != nil {
					return err
				}
			}
			if _, err := e.w.WriteBits(bit.String(`\ufffd`)); err != nil {
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
				if _, err := e.w.WriteBits(bit.String(s[start:i])); err != nil {
					return err
				}
			}
			if _, err := e.w.WriteBits(bit.String(`\u202`)); err != nil {
				return err
			}
			if _, err := e.w.WriteBits(bit.Bytes(hex[c&0xF])); err != nil {
				return err
			}
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		if _, err := e.w.WriteBits(bit.String(s[start:])); err != nil {
			return err
		}
	}
	if _, err := e.w.WriteBits(bit.Bytes('"')); err != nil {
		return err
	}
	return nil
}

// base64 is an efficient base64 encoder, adapted from encoding/base64.
func (m *Encoder) base64(b []byte) error {
	const std = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	if len(b) == 0 {
		_, err := m.w.WriteBits(bit.String(`"null"`))
		return err
	}
	if _, err := m.w.WriteBits(bit.Bytes('"')); err != nil {
		return err
	}
	// Encode full quanta.
	n := len(b) / 3 * 3
	for i := 0; i < n; i += 3 {
		// We could use binary.BigEndian.Uint32 here, but it's overkill.
		x := uint32(b[i+0])<<16 | uint32(b[i+1])<<8 | uint32(b[i+2])
		if _, err := m.w.WriteBits(bit.Bytes(std[x>>18])); err != nil {
			return err
		}
		if _, err := m.w.WriteBits(bit.Bytes(std[x>>12&0x3f])); err != nil {
			return err
		}
		if _, err := m.w.WriteBits(bit.Bytes(std[x>>6&0x3f])); err != nil {
			return err
		}
		if _, err := m.w.WriteBits(bit.Bytes(std[x&0x3f])); err != nil {
			return err
		}
	}
	remain := len(b) - n
	if remain == 0 {
		if _, err := m.w.WriteBits(bit.Bytes('"')); err != nil {
			return err
		}
		return nil
	}
	// Encode partial quanta.
	switch len(b) - n {
	case 1:
		x := uint32(b[n+0])
		if _, err := m.w.WriteBits(bit.Bytes(std[x>>2])); err != nil {
			return err
		}
		if _, err := m.w.WriteBits(bit.Bytes(std[x<<4&0x3f])); err != nil {
			return err
		}
		if _, err := m.w.WriteBits(bit.String("==")); err != nil {
			return err
		}
	case 2:
		x := uint32(b[n+0])<<8 | uint32(b[n+1])
		if _, err := m.w.WriteBits(bit.Bytes(std[x>>10])); err != nil {
			return err
		}
		if _, err := m.w.WriteBits(bit.Bytes(std[x>>4&0x3f])); err != nil {
			return err
		}
		if _, err := m.w.WriteBits(bit.Bytes(std[x<<2&0x3f])); err != nil {
			return err
		}
		if _, err := m.w.WriteBits(bit.Bytes('=')); err != nil {
			return err
		}
	}
	if _, err := m.w.WriteBits(bit.Bytes('"')); err != nil {
		return err
	}
	return nil
}
