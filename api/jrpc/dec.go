package jrpc

import (
	"errors"
	"io"
	"reflect"
	_ "unsafe"
)

type Decoder struct {
	r io.Reader

	segment [256]byte
	prev    *[256]byte

	head, size int
}

func NewDecoder(r io.Reader) Decoder {
	return Decoder{r: r}
}

//go:linkname read runtime.link/api/jrpc.read_impl
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

func (d *Decoder) scan_byte() (b byte, err error) {
	if d.head == d.size {
		n, err := read(d.r, d.segment[d.head:])
		if err != nil {
			return 0, err
		}
		d.head = 0
		d.size += n
	}
	b = d.segment[d.head]
	d.head++
	if d.head == 256 {
		d.head = 0
		d.size = 0
		d.prev = new([256]byte)
		copy(d.prev[:], d.segment[:])
	}
	return b, nil
}

func (d *Decoder) decode(rvalue reflect.Value) error {
	rtype := rvalue.Type()
	switch rtype.Kind() {
	case reflect.Bool:
		var True = [4]byte{'t', 'r', 'u', 'e'}
		var False = [5]byte{'f', 'a', 'l', 's', 'e'}
		for i := range len(False) {
			char, err := d.scan_byte()
			if err != nil {
				return err
			}
			if char != False[i] && (i < len(True) && char != True[i]) {
				return errors.New("expected boolean")
			}
			if i == 3 && char == 'e' {
				rvalue.SetBool(true)
				return nil
			}
		}
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
			if char == ',' {
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
			if char == ',' {
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
		var whole int64
		var fract uint64
		var negative bool
		var decimals uint
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
			if char == '.' {
				decimals = 1
				continue
			}
			if char == ',' {
				break
			}
			if char < '0' || char > '9' {
				return errors.New("expected float")
			}
			if decimals == 0 {
				whole = whole*10 + int64(char-'0')
			} else {
				fract = fract*10 + uint64(char-'0')
				decimals *= 10
			}
		}
		if negative {
			whole = -whole
		}
		rvalue.SetFloat(float64(whole) + float64(fract)/float64(decimals))
		return nil
	case reflect.String:
		char, err := d.scan_byte()
		if err != nil {
			return err
		}
		if char != '"' {
			return errors.New("expected string")
		}
		for {

		}
	default:
		return errors.New("jrpc: unsupported type")
	}
}
