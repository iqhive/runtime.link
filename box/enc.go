package box

import (
	"encoding/binary"
	"reflect"

	"runtime.link/api/xray"
	"runtime.link/ram"
)

// Encoder for encoding values in box format, values are
// serialised based on their in-memory representation.
type Encoder struct {
	w ram.Writer

	packed bool // whether to optimally compress each value.

	native Binary
	config Binary

	first bool

	rtypes uint16
	lookup map[reflect.Type]cache
}

// NewEncoder returns a new [Encoder] that writes to the
// specified writer.
func NewEncoder(w ram.Writer) *Encoder {
	native := NativeBinary()
	return &Encoder{
		w:      w,
		rtypes: 1,
		first:  true,
		native: native,
		config: native,
		lookup: make(map[reflect.Type]cache),
	}
}

// SetBinary sets the [Binary] encoding to use.
func (enc *Encoder) SetBinary(binary Binary) {
	enc.config = binary
	enc.SetPacked(enc.packed)
}

// SetPacked determines whether to optimally pack values, this is
// more CPU intensive but will result in smaller messages.
func (enc *Encoder) SetPacked(packed bool) {
	enc.packed = packed
	enc.config |= MemorySize4
}

// Encode writes the specified value to the writer in box
// format.
func (enc *Encoder) Encode(val any) (err error) {
	if enc.first {
		_, err := enc.w.Write([]byte{'B', 'O', 'X', '1'})
		if err != nil {
			return err
		}
		enc.first = false
	}
	rtype := reflect.TypeOf(val)
	value := reflect.ValueOf(val)
	if !value.CanAddr() {
		value = reflect.New(rtype).Elem()
		value.Set(reflect.ValueOf(val))
	}
	var specific template
	if n, ok := enc.lookup[rtype]; ok {
		specific = n.enc
		return enc.box(n.def, 0, 0)
	} else {
		specific, err = enc.basic(1, rtype, value)
		if err != nil {
			return xray.New(err)
		}
		enc.lookup[rtype] = cache{def: enc.rtypes, enc: specific}
		enc.rtypes++
	}
	if _, err := enc.w.Write([]byte{0}); err != nil {
		return xray.New(err)
	}
	if err := enc.value(specific, value); err != nil {
		return xray.New(err)
	}
	return nil
}

func (enc Encoder) box(n uint16, kind Object, T Schema) error {
	var buf [4]byte
	if n < 31 {
		buf[0] = byte(n) | byte(kind)
		_, err := enc.w.Write(buf[:1])
		return xray.New(err)
	}
	buf[0] = 31 | byte(kind)
	if enc.config&BinarySchema != 0 && (kind != ObjectStruct && T == 0) {
		if n < 127 {
			buf[1] = byte(n) | byte(T)
			_, err := enc.w.Write(buf[:2])
			return xray.New(err)
		}
		buf[1] = 127 | byte(T)
		if enc.native&BinaryEndian != 0 {
			binary.BigEndian.PutUint16(buf[2:], n)
		} else {
			binary.LittleEndian.PutUint16(buf[2:], n)
		}
		_, err := enc.w.Write(buf[:])
		return xray.New(err)
	} else {
		if enc.native&BinaryEndian != 0 {
			binary.BigEndian.PutUint16(buf[1:3], n)
		} else {
			binary.LittleEndian.PutUint16(buf[1:3], n)
		}
		_, err := enc.w.Write(buf[:3])
		return xray.New(err)
	}
}

func (enc Encoder) end() error {
	return enc.box(0, ObjectStruct, 0)
}
