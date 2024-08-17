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

	box uint64 // sequence tracker.

	rtypes uint64
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
		return enc.object(n.def, 0, 0, "")
	} else {
		specific, err = enc.basic(1, rtype, value, "")
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

func (enc Encoder) object(box uint64, kind Object, schema Schema, hint string) error {
	if box == 0 { // box sequence tracking.
		enc.box++
		box = enc.box
	} else {
		if box == enc.box+1 {
			box = 0
		} else {
			enc.box = box
		}
	}
	var buf = make([]byte, 0, 18)
	if box < 31 {
		buf = append(buf, byte(box)|byte(kind))
	} else {
		buf = append(buf, 31|byte(kind))
		if enc.native&BinaryEndian != 0 {
			buf = binary.BigEndian.AppendUint16(buf, uint16(box-30))
		} else {
			buf = binary.LittleEndian.AppendUint16(buf, uint16(box-30))
		}
	}
	if enc.config&BinarySchema != 0 {
		if len(hint) < 31 {
			buf = append(buf, byte(len(hint))|byte(schema))
		} else {
			buf = append(buf, 31|byte(schema))
			if enc.native&BinaryEndian != 0 {
				buf = binary.BigEndian.AppendUint16(buf, uint16(len(hint)-30))
			} else {
				buf = binary.LittleEndian.AppendUint16(buf, uint16(len(hint)-30))
			}
		}
	}
	if _, err := enc.w.Write(buf); err != nil {
		return err
	}
	if enc.config&BinarySchema != 0 {
		if _, err := enc.w.Write([]byte(hint)); err != nil {
			return err
		}
	}
	return nil
}

func (enc Encoder) end() error {
	_, err := enc.w.Write([]byte{byte(ObjectStruct)})
	return err
}
