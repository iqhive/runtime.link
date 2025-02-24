package box

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"

	"runtime.link/api/xray"
)

// Encoder for encoding values in box format, values are
// serialised based on their in-memory representation.
type Encoder struct {
	w *bufio.Writer

	native Binary
	binary Binary

	ram uintptr
	ptr []uintptr

	first bool
}

// NewEncoder returns a new [Encoder] that writes to the
// specified writer.
func NewEncoder(w io.Writer) *Encoder {
	native := NativeBinary()
	return &Encoder{
		w:      bufio.NewWriter(w),
		first:  true,
		native: native,
		binary: native,
	}
}

// SetBinary sets the [Binary] encoding to use.
func (enc *Encoder) SetBinary(binary Binary) { enc.binary = binary }

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
	if err := enc.w.WriteByte(byte(enc.binary)); err != nil {
		return err
	}
	enc.ram = 0
	enc.ptr = enc.ptr[0:0]
	if err := enc.memory(rtype, value); err != nil {
		return xray.New(err)
	}
	if err := enc.object(1, true, rtype, value, 0, rtype.Name()); err != nil {
		return xray.New(err)
	}
	if err := enc.w.WriteByte(0); err != nil {
		return err
	}
	if err := enc.pointers(value); err != nil {
		return xray.New(err)
	}
	if err := enc.x(value); err != nil {
		return xray.New(err)
	}
	return xray.New(enc.w.Flush())
}

func (enc *Encoder) box(n uint16, kind Object, schema Schema, hint string) error {
	var buf = make([]byte, 0, 18)
	if n < 31 {
		buf = append(buf, byte(n)|byte(kind))
	} else {
		buf = append(buf, 31|byte(kind))
		if enc.native&BinaryEndian != 0 {
			buf = binary.BigEndian.AppendUint16(buf, uint16(n-30))
		} else {
			buf = binary.LittleEndian.AppendUint16(buf, uint16(n-30))
		}
	}
	if enc.binary&BinarySchema != 0 {
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
	if enc.binary&BinarySchema != 0 {
		if _, err := enc.w.Write([]byte(hint)); err != nil {
			return err
		}
	}
	return nil
}

func (enc *Encoder) end() error {
	_, err := enc.w.Write([]byte{byte(ObjectStruct)})
	return err
}

func (enc *Encoder) packed() bool { return enc.binary&BinaryPacked != 0 }

func (enc *Encoder) sizeof(n uintptr) Object {
	switch n {
	case 1:
		return ObjectBytes1
	case 2:
		return ObjectBytes2
	case 4:
		return ObjectBytes4
	case 8:
		return ObjectBytes8
	default:
		panic("invalid size")
	}
}
func (enc *Encoder) memory(rtype reflect.Type, value reflect.Value) error {
	switch rtype.Kind() {
	case reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if err := enc.memory(rtype.Elem(), value.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		if value.Len() == 0 {
			return nil
		}
		keys := value.MapKeys()
		for i := 0; i < len(keys); i++ {
			if err := enc.memory(rtype.Key(), keys[i]); err != nil {
				return err
			}
			if err := enc.memory(rtype.Elem(), value.MapIndex(keys[i])); err != nil {
				return err
			}
		}
	case reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		if err := enc.memory(rtype.Elem(), value.Elem()); err != nil {
			return err
		}
	case reflect.Slice:
		length := value.Len()
		if length == 0 {
			return nil
		}
		for i := 0; i < length; i++ {
			if err := enc.memory(rtype.Elem(), value.Index(i)); err != nil {
				return err
			}
		}
	case reflect.String:
		length := value.Len()
		if length == 0 {
			return nil
		}
		if err := enc.box(uint16(length), ObjectRepeat, SchemaUnicode, ""); err != nil {
			return err
		}
		if err := enc.box(0, ObjectBytes1, SchemaNatural, ""); err != nil {
			return err
		}
		return nil
	case reflect.Struct:
		for i := 0; i < rtype.NumField(); i++ {
			if err := enc.memory(rtype.Field(i).Type, value.Field(i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (enc *Encoder) object(box uint16, direct bool, rtype reflect.Type, value reflect.Value, offset uintptr, hint string) error {
	if enc.packed() && value.IsZero() {
		return nil
	}
	switch rtype.Kind() {
	case reflect.Bool:
		if err := enc.box(box, ObjectBytes1, SchemaBoolean, hint); err != nil {
			return err
		}
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		size := rtype.Size()
		if enc.packed() {
			val := value.Int()
			if val <= math.MaxInt8 && val >= math.MinInt8 {
				size = 1
			}
			if val <= math.MaxUint16 && val >= math.MinInt16 {
				size = 2
			}
			if val <= math.MaxUint32 && val >= math.MinInt32 {
				size = 4
			}
		}
		if err := enc.box(box, enc.sizeof(size), SchemaInteger, hint); err != nil {
			return err
		}
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
		size := rtype.Size()
		if enc.packed() {
			val := value.Uint()
			if val <= math.MaxUint8 {
				size = 1
			}
			if val <= math.MaxUint16 {
				size = 2
			}
			if val <= math.MaxUint32 {
				size = 4
			}
		}
		if err := enc.box(box, enc.sizeof(size), SchemaNatural, hint); err != nil {
			return err
		}
	case reflect.Float32, reflect.Float64:
		size := rtype.Size()
		if err := enc.box(box, enc.sizeof(size), SchemaIEEE754, hint); err != nil {
			return err
		}
	case reflect.Complex64, reflect.Complex128:
		size := rtype.Size()
		if err := enc.box(2, ObjectRepeat, SchemaComplex, hint); err != nil {
			return err
		}
		if err := enc.box(box, enc.sizeof(size/2), SchemaComplex, ""); err != nil {
			return err
		}
	case reflect.Array:
		var size = rtype.Len()
		if size > math.MaxUint16 {
			return errors.New("array size too large")
		}
		if err := enc.box(uint16(size), ObjectRepeat, SchemaOrdered, hint); err != nil {
			return err
		}
		if enc.packed() {
			return fmt.Errorf("packed arrays not supported")
		}
		if err := enc.object(box, false, rtype.Elem(), reflect.Value{}, offset, hint); err != nil {
			return err
		}
	case reflect.Struct:
		if !direct {
			if err := enc.box(box, ObjectStruct, SchemaSourced, hint); err != nil {
				return err
			}
		}
		for i := 0; i < rtype.NumField(); i++ {
			field := rtype.Field(i)
			if err := enc.object(uint16(i), false, field.Type, value.Field(i), offset+field.Offset, field.Name); err != nil {
				return err
			}
		}
		if !direct {
			if err := enc.box(0, ObjectIgnore, SchemaSourced, ""); err != nil {
				return err
			}
		}
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.UnsafePointer:
		return fmt.Errorf("unsupported type: %v", rtype)
	case reflect.Map:
		if err := enc.box(box, ObjectMemory, SchemaPointer, hint); err != nil {
			return err
		}
	case reflect.Pointer:
		if err := enc.box(box, ObjectMemory, SchemaPointer, hint); err != nil {
			return err
		}
	case reflect.String:
		if err := enc.box(2, ObjectRepeat, SchemaOrdered, ""); err != nil {
			return err
		}
		if err := enc.box(box, ObjectMemory, 0, ""); err != nil {
			return err
		}
	}
	return nil
}
