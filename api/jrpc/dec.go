package jrpc

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	_ "unsafe"
)

type Decoder struct {
	r io.Reader

	segment [256]byte
	readErr error

	head, size int
}

func NewDecoder(r io.Reader) Decoder {
	return Decoder{r: r}
}

//go:linkname read github.com/iqhive/runtime.link/api/jrpc.read_impl
//go:noescape
func read(w io.Reader, buf []byte) (int, error)

func read_impl(r io.Reader, buf []byte) (int, error) {
	return r.Read(buf)
}

func (d *Decoder) Decode(v any) error {
	rvalue := reflect.ValueOf(v)
	if rvalue.Kind() != reflect.Ptr {
		return errors.New("jrpc: Decode requires a pointer")
	}
	return d.decode(rvalue.Elem())
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

func (d *Decoder) decode(rvalue reflect.Value) error {
	if err := d.skipWhitespace(); err != nil {
		return err
	}
	rtype := rvalue.Type()
	switch rtype.Kind() {
	case reflect.Bool:
		char, err := d.peek_byte()
		if err != nil {
			return err
		}
		literal := "false"
		if char == 't' {
			literal = "true"
		}
		for i := range len(literal) {
			char, err := d.scan_byte()
			if err != nil {
				return err
			}
			if char != literal[i] {
				return errors.New("expected boolean")
			}
		}
		rvalue.SetBool(literal == "true")
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var register int64
		var negative bool
		for {
			char, err := d.scan_byte()
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if char == '-' {
				negative = true
				continue
			}
			if char == ',' || char == '}' || char == ']' || char == ' ' || char == '\t' || char == '\n' || char == '\r' {
				// Put back the terminator character for the parent context
				d.head--
				break
			}
			if char < '0' || char > '9' {
				return errors.New("expected integer")
			}
			register = register*10 + int64(char-'0')
		}
		if negative {
			register = -register
		}
		rvalue.SetInt(register)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var register uint64
		for {
			char, err := d.scan_byte()
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if char == ',' || char == '}' || char == ']' || char == ' ' || char == '\t' || char == '\n' || char == '\r' {
				// Put back the terminator character for the parent context
				d.head--
				break
			}
			if char < '0' || char > '9' {
				return errors.New("expected unsigned integer")
			}
			register = register*10 + uint64(char-'0')
		}
		rvalue.SetUint(register)
		return nil
	case reflect.Float32, reflect.Float64:
		var number []byte
		for {
			char, err := d.peek_byte()
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if char == ',' || char == '}' || char == ']' || char == ' ' || char == '\t' || char == '\n' || char == '\r' {
				break
			}
			number = append(number, char)
			d.head++
		}
		return json.Unmarshal(number, rvalue.Addr().Interface())
	case reflect.String:
		char, err := d.scan_byte()
		if err != nil {
			return err
		}
		if char != '"' {
			return errors.New("expected string")
		}
		var buffer []byte
		for {
			char, err := d.scan_byte()
			if err != nil {
				return err
			}
			if char == '"' {
				rvalue.SetString(string(buffer))
				return nil
			}
			if char == '\\' {
				escape, err := d.scan_byte()
				if err != nil {
					return err
				}
				switch escape {
				case '"', '\\', '/':
					buffer = append(buffer, escape)
				case 'b':
					buffer = append(buffer, '\b')
				case 'f':
					buffer = append(buffer, '\f')
				case 'n':
					buffer = append(buffer, '\n')
				case 'r':
					buffer = append(buffer, '\r')
				case 't':
					buffer = append(buffer, '\t')
				case 'u':
					var hexCode [4]byte
					for i := 0; i < 4; i++ {
						hexChar, err := d.scan_byte()
						if err != nil {
							return err
						}
						hexCode[i] = hexChar
					}
					// Parse the 4-digit hex code
					var runeValue rune
					for i := 0; i < 4; i++ {
						runeValue = runeValue << 4
						if hexCode[i] >= '0' && hexCode[i] <= '9' {
							runeValue += rune(hexCode[i] - '0')
						} else if hexCode[i] >= 'a' && hexCode[i] <= 'f' {
							runeValue += rune(hexCode[i] - 'a' + 10)
						} else if hexCode[i] >= 'A' && hexCode[i] <= 'F' {
							runeValue += rune(hexCode[i] - 'A' + 10)
						} else {
							return errors.New("invalid hex digit in unicode escape")
						}
					}
					buffer = append(buffer, string(runeValue)...)
				default:
					return errors.New("invalid escape sequence")
				}
			} else {
				buffer = append(buffer, char)
			}
		}
	case reflect.Map:
		if rvalue.IsNil() {
			rvalue.Set(reflect.MakeMap(rtype))
		}
		char, err := d.scan_byte()
		if err != nil {
			return err
		}
		if char != '{' {
			return errors.New("expected object")
		}
		if err := d.skipWhitespace(); err != nil {
			return err
		}
		if char == '}' {
			// Empty object
			return nil
		}

		for {
			// Read the key (must be a string)
			keyValue := reflect.New(rtype.Key()).Elem()
			if err := d.decode(keyValue); err != nil {
				return err
			}

			// Skip whitespace and expect colon
			if err = d.skipWhitespace(); err != nil {
				return err
			}
			char, err := d.scan_byte()
			if err != nil {
				return err
			}

			// Expect colon
			if char != ':' {
				return errors.New("expected colon after object key")
			}

			// Skip whitespace before value
			if err = d.skipWhitespace(); err != nil {
				return err
			}

			// Read the value
			elemValue := reflect.New(rtype.Elem()).Elem()
			if err := d.decode(elemValue); err != nil {
				return err
			}

			// Set the key-value pair in the map
			rvalue.SetMapIndex(keyValue, elemValue)

			// Skip whitespace
			if err = d.skipWhitespace(); err != nil {
				return err
			}

			char, err = d.scan_byte()
			if err != nil {
				return err
			}
			// Check for end of object or comma
			if char == '}' {
				return nil
			}
			if char != ',' {
				return errors.New("expected comma or closing brace after object value")
			}

			// Skip whitespace after comma
			if err = d.skipWhitespace(); err != nil {
				return err
			}

		}
	case reflect.Struct:
		char, err := d.scan_byte()
		if err != nil {
			return err
		}
		if char != '{' {
			return errors.New("expected object")
		}
		// Create a map of field names to field indices
		fields := make(map[string]int)
		for i := 0; i < rtype.NumField(); i++ {
			field := rtype.Field(i)
			name := field.Name

			// Check for json tag
			if tag := field.Tag.Get("json"); tag != "" {
				if comma := strings.IndexByte(tag, ','); comma >= 0 {
					name = tag[:comma]
				} else {
					name = tag
				}
				if name == "-" {
					continue // Skip this field
				}
			}

			fields[name] = i
		}

		// Skip whitespace
		if err = d.skipWhitespace(); err != nil {
			return err
		}
		char, err = d.peek_byte()
		if err != nil {
			return err
		}
		if char == '}' {
			// Empty object
			return nil
		}
		for {
			// Read the key (must be a string)
			var key string
			keyValue := reflect.ValueOf(&key).Elem()
			if err := d.decode(keyValue); err != nil {
				return err
			}

			if err = d.skipWhitespace(); err != nil {
				return err
			}
			// Skip whitespace and expect colon
			char, err = d.scan_byte()
			if err != nil {
				return err
			}

			// Expect colon
			if char != ':' {
				return errors.New("expected colon after object key")
			}

			// Skip whitespace before value
			if err = d.skipWhitespace(); err != nil {
				return err
			}

			// Find the field
			if fieldIndex, ok := fields[key]; ok {
				// Decode into the field
				if err := d.decode(rvalue.Field(fieldIndex)); err != nil {
					return err
				}
			} else {
				// Skip the value
				var dummy interface{}
				dummyValue := reflect.ValueOf(&dummy).Elem()
				if err := d.decode(dummyValue); err != nil {
					return err
				}
			}

			// Skip whitespace
			if err = d.skipWhitespace(); err != nil {
				return err
			}

			if char, err = d.scan_byte(); err != nil {
				return err
			}

			// Check for end of object or comma
			if char == '}' {
				return nil
			}
			if char != ',' {
				return errors.New("expected comma or closing brace after object value")
			}
		}
	case reflect.Array, reflect.Slice:
		char, err := d.scan_byte()
		if err != nil {
			return err
		}
		if char != '[' {
			return errors.New("expected array")
		}

		// For slices, initialize with zero length but non-zero capacity
		if rtype.Kind() == reflect.Slice {
			rvalue.Set(reflect.MakeSlice(rtype, 0, 4))
		}

		// Skip whitespace
		if err = d.skipWhitespace(); err != nil {
			return err
		}
		char, err = d.peek_byte()
		if err != nil {
			return err
		}
		if char == ']' {
			// Empty array
			return nil
		}

		index := 0
		for {
			// For arrays, check bounds
			if rtype.Kind() == reflect.Array && index >= rvalue.Len() {
				return errors.New("array index out of bounds")
			}

			// For slices, grow as needed
			if rtype.Kind() == reflect.Slice {
				if index >= rvalue.Cap() {
					newCap := rvalue.Cap() * 2
					if newCap == 0 {
						newCap = 4
					}
					newSlice := reflect.MakeSlice(rtype, rvalue.Len(), newCap)
					reflect.Copy(newSlice, rvalue)
					rvalue.Set(newSlice)
				}
				if index >= rvalue.Len() {
					rvalue.SetLen(index + 1)
				}
			}

			// Decode the element
			var elemValue reflect.Value
			if rtype.Kind() == reflect.Array {
				elemValue = rvalue.Index(index)
			} else {
				elemValue = rvalue.Index(index)
			}

			if err := d.decode(elemValue); err != nil {
				return err
			}

			index++

			// Skip whitespace
			if err = d.skipWhitespace(); err != nil {
				return err
			}
			char, err = d.scan_byte()
			if err != nil {
				return err
			}

			// Check for end of array or comma
			if char == ']' {
				return nil
			}
			if char != ',' {
				return errors.New("expected comma or closing bracket after array element")
			}
		}
	case reflect.Interface, reflect.Pointer:
		char, err := d.peek_byte()
		if err != nil {
			return err
		}
		if char == 'n' {
			for _, want := range []byte("null") {
				char, err := d.scan_byte()
				if err != nil {
					return err
				}
				if char != want {
					return errors.New("expected null")
				}
			}
			rvalue.Set(reflect.Zero(rtype))
			return nil
		}
		if rvalue.IsNil() {
			if rtype.Kind() == reflect.Pointer {
				rvalue.Set(reflect.New(rtype.Elem()))
			} else {
				return errors.New("cannot decode non-null value into nil interface")
			}
		}
		return d.decode(rvalue.Elem())

	default:
		return errors.New("jrpc: unsupported type")
	}
}
