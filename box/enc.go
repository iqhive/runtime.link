package box

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"time"
	"unsafe"

	"runtime.link/api/xray"
	"runtime.link/ram"
)

// Encoder for encoding values in box format, values are
// serialised based on their in-memory representation.
type Encoder struct {
	w ram.Writer

	big   big       // little or big (endian?
	epoch time.Time // epoch used for elapsed time

	first bool

	schema bool // include rich type information?

	rtypes uint16
	lookup map[reflect.Type]cache
}

type cache struct {
	def uint16
	ptr bool
}

// NewEncoder returns a new [Encoder] that writes to the
// specified writer.
func NewEncoder(w ram.Writer) *Encoder {
	return &Encoder{
		w:      w,
		rtypes: 1,
		epoch:  time.Unix(0, 0),
		first:  true,
		lookup: make(map[reflect.Type]cache),
	}
}

// Encode writes the specified value to the writer in box
// format.
func (enc *Encoder) Encode(val any) (err error) {
	if enc.first {
		_, err := enc.w.Write([]byte{'b', 'o', 'x', byte(metaSchema)})
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
	var ptr bool
	if n, ok := enc.lookup[rtype]; ok {
		ptr = n.ptr
		return enc.box(n.def, kindLookup, 0)
	} else {
		ptr, err = enc.basic(1, rtype)
		if err != nil {
			return xray.New(err)
		}
		enc.lookup[rtype] = cache{def: enc.rtypes, ptr: ptr}
		enc.rtypes++
	}
	if err := enc.value(ptr, rtype, value); err != nil {
		return xray.New(err)
	}
	return nil
}

func (enc Encoder) box(n uint16, kind box, T uno) error {
	var buf [4]byte
	if n < 31 {
		buf[0] = byte(n) | byte(kind)
		_, err := enc.w.Write(buf[:1])
		return err
	}
	buf[0] = 31 | byte(kind)
	if enc.schema && kind != kindLookup && kind != kindStatic {
		if n < 127 {
			buf[1] = byte(n) | byte(T)
			_, err := enc.w.Write(buf[:2])
			return err
		}
		buf[1] = 127 | byte(T)
		if enc.big {
			binary.BigEndian.PutUint16(buf[2:], n)
		} else {
			binary.LittleEndian.PutUint16(buf[2:], n)
		}
		_, err := enc.w.Write(buf[:])
		return err
	} else {
		if enc.big {
			binary.BigEndian.PutUint16(buf[1:3], n)
		} else {
			binary.LittleEndian.PutUint16(buf[1:3], n)
		}
		_, err := enc.w.Write(buf[:3])
		return err
	}
}

func (enc Encoder) end() error {
	_, err := enc.w.Write([]byte{0})
	return err
}

func (enc Encoder) basic(box uint16, rtype reflect.Type) (bool, error) {
	switch rtype.Kind() {
	case reflect.Bool:
		return false, enc.box(box, kindBytes1, typeBoolean)
	case reflect.Uint8:
		return false, enc.box(box, kindBytes1, typeNatural)
	case reflect.Int8:
		return false, enc.box(box, kindBytes1, typeInteger)
	case reflect.Uint16:
		return false, enc.box(box, kindBytes2, typeNatural)
	case reflect.Int16:
		return false, enc.box(box, kindBytes2, typeInteger)
	case reflect.Uint32:
		return false, enc.box(box, kindBytes4, typeNatural)
	case reflect.Int32:
		return false, enc.box(box, kindBytes4, typeInteger)
	case reflect.Float32:
		return false, enc.box(box, kindBytes4, typeIEEE754)
	case reflect.Uint64:
		return false, enc.box(box, kindBytes8, typeNatural)
	case reflect.Int64:
		return false, enc.box(box, kindBytes8, typeInteger)
	case reflect.Float64:
		return false, enc.box(box, kindBytes8, typeIEEE754)
	case reflect.Int:
		if rtype.Size() == 8 {
			return false, enc.box(box, kindBytes8, typeInteger)
		}
		return false, enc.box(box, kindBytes4, typeInteger)
	case reflect.Uint, reflect.Uintptr:
		if rtype.Size() == 8 {
			return false, enc.box(box, kindBytes8, typeNatural)
		}
		return false, enc.box(box, kindBytes4, typeNatural)
	case reflect.Complex64:
		if err := enc.box(2, kindRepeat, typeOrdered); err != nil {
			return false, err
		}
		return false, enc.box(box, kindBytes4, typeIEEE754)
	case reflect.Complex128:
		if err := enc.box(2, kindRepeat, typeOrdered); err != nil {
			return false, err
		}
		return false, enc.box(box, kindBytes8, typeIEEE754)
	case reflect.Array:
		size := rtype.Len()
		if size > math.MaxUint16 {
			return false, fmt.Errorf("array size %d exceeds box maximum %d", size, math.MaxUint16)
		}
		if err := enc.box(uint16(size), kindRepeat, typeOrdered); err != nil {
			return false, err
		}
		return enc.basic(box, rtype.Elem())
	case reflect.String:
		if err := enc.box(2, kindStruct, typeUnicode); err != nil {
			return true, err
		}
		if err := enc.box(1, kindBytes8, typePointer); err != nil {
			return true, err
		}
		if err := enc.box(2, kindBytes8, typeInteger); err != nil {
			return true, err
		}
		return true, enc.end()
	case reflect.Interface:
		if err := enc.box(2, kindStruct, typeDynamic); err != nil {
			return true, err
		}
		if err := enc.box(1, kindBytes8, typePointer); err != nil {
			return true, err
		}
		if err := enc.box(2, kindBytes8, typePointer); err != nil {
			return true, err
		}
		return true, enc.end()
	case reflect.Slice:
		if err := enc.box(2, kindStruct, typeOrdered); err != nil {
			return true, err
		}
		if err := enc.box(1, kindBytes8, typePointer); err != nil {
			return true, err
		}
		if err := enc.box(2, kindBytes8, typeInteger); err != nil {
			return true, err
		}
		if err := enc.box(3, kindBytes8, typeInteger); err != nil {
			return true, err
		}
		return true, enc.end()
	case reflect.Map:
		if err := enc.box(2, kindStruct, typeMapping); err != nil {
			return true, err
		}
		if err := enc.box(1, kindBytes8, typePointer); err != nil {
			return true, err
		}
		return true, enc.end()
	case reflect.Struct:
		if err := enc.box(box, kindStruct, typeDefined); err != nil {
			return false, err
		}
		var ptr bool
		var err error
		var offset uintptr
		var sizing uintptr
		padding := func(reached uintptr) error {
			if reached > offset+sizing {
				for i := reached - (offset + sizing); i > 0; {
					switch {
					case i%8 == 0:
						if err := enc.box(0, kindBytes8, typePadding); err != nil {
							return err
						}
						i -= 8
					case i%4 == 0:
						if err := enc.box(0, kindBytes4, typePadding); err != nil {
							return err
						}
						i -= 4
					case i%2 == 0:
						if err := enc.box(0, kindBytes2, typePadding); err != nil {
							return err
						}
						i -= 2
					case i%1 == 0:
						if err := enc.box(0, kindBytes1, typePadding); err != nil {
							return err
						}
						i--
					}
				}
			}
			return nil
		}
		for i := 0; i < rtype.NumField(); i++ {
			field := rtype.Field(i)
			if err := padding(field.Offset); err != nil {
				return ptr, err
			}
			offset = field.Offset
			sizing = field.Type.Size()
			box := uint16(i)
			tag, ok := field.Tag.Lookup("box")
			if ok {
				if _, err := fmt.Sscanf(tag, "%d", &box); err != nil {
					return ptr, fmt.Errorf("invalid box tag %q: %v", tag, err)
				}
			}
			ptr, err = enc.basic(box, field.Type)
			if err != nil {
				return ptr, err
			}
		}
		if err := padding(rtype.Size()); err != nil {
			return ptr, err
		}
		if err := enc.end(); err != nil {
			return ptr, err
		}
		return ptr, nil
	case reflect.Pointer, reflect.UnsafePointer:
		if rtype.Size() == 8 {
			return true, enc.box(box, kindBytes8, typePointer)
		}
		return true, enc.box(box, kindBytes4, typePointer)
	case reflect.Chan:
		return true, enc.box(box, kindBytes8, typeChannel)
	case reflect.Func:
		if err := enc.box(2, kindStruct, typeProgram); err != nil {
			return true, err
		}
		if err := enc.box(1, kindBytes8, typePointer); err != nil {
			return true, err
		}
		return true, enc.end()
	}
	return false, fmt.Errorf("unsupported type %v", rtype)
}

func (enc Encoder) value(hasPointers bool, rtype reflect.Type, value reflect.Value) error {
	raw := unsafe.Slice((*byte)(value.Addr().UnsafePointer()), rtype.Size())
	if !hasPointers {
		if _, err := enc.w.Write(raw); err != nil {
			return err
		}
		if err := enc.box(0, kindStatic, 0); err != nil {
			return err
		}
		if err := enc.box(0, kindStatic, 0); err != nil {
			return err
		}
		return nil
	}
	var buf = make([]byte, rtype.Size())
	copy(buf, raw)
	return fmt.Errorf("nested pointers not yet supported")
}
