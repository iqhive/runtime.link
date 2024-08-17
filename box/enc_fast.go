package box

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"unsafe"

	"runtime.link/api/xray"
	"runtime.link/ram"
)

// template should be generated for each Go type, it represents the translation
// between the Go type and the box payload for it.
type template struct {
	fast rcopy
	slow []rcopy
	then []after
}

// rcopy is a slice to copy from the go type to the
// box payload.
type rcopy struct {
	offset int64 // offset in the go type or static value, if negative, treat as -offset-memory
	length int64 // if negative, then write -length bytes of 'offset'?
	string string
}

func (op rcopy) copy(dst ram.Writer, src unsafe.Pointer, memory *int) error {
	if op == (rcopy{}) {
		return nil
	}
	if op.string != "" {
		_, err := dst.Write([]byte(op.string))
		return err
	}
	if op.offset < 0 {
		op.offset = -op.offset
		op.offset = int64(*memory)
		*memory += int(op.offset)
	}
	if op.length < 0 {
		src = unsafe.Pointer(&op.offset)
		op.length = -op.length
	} else {
		src = unsafe.Add(src, op.offset)
	}
	_, err := dst.Write(unsafe.Slice((*byte)(src), int(op.length)))
	return err
}

// after is a value to encode to the memory after the initial
// object memory is written.
type after struct {
	offset uintptr
	handle reflect.Type
}

type cache struct {
	def uint64
	enc template
}

type sizer struct {
	bytes uint
	addrs uint
	alloc uint
}

func (s *sizer) add(b sizer) {
	s.bytes += b.bytes
	s.addrs += b.addrs
	s.alloc += b.alloc
}

// sizer returns the smallest packed size for the given type.
func (enc Encoder) sizer(rtype reflect.Type, value reflect.Value) sizer {
	if value.IsZero() {
		return sizer{}
	}
	switch rtype.Kind() {
	case reflect.Bool, reflect.Uint8, reflect.Int8:
		return sizer{1 + 1, 0, 0}
	case reflect.Int16, reflect.Int32, reflect.Int64:
		val := value.Int()
		if val <= math.MaxInt8 && val >= math.MinInt8 {
			return sizer{1 + 1, 0, 0}
		}
		if val <= math.MaxUint16 && val >= math.MinInt16 {
			return sizer{1 + 2, 0, 0}
		}
		if val <= math.MaxUint32 && val >= math.MinInt32 {
			return sizer{1 + 4, 0, 0}
		}
		return sizer{1 + 8, 0, 0}
	case reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
		val := value.Uint()
		if val <= math.MaxUint8 {
			return sizer{1 + 1, 0, 0}
		}
		if val <= math.MaxUint16 {
			return sizer{1 + 2, 0, 0}
		}
		if val <= math.MaxUint32 {
			return sizer{1 + 4, 0, 0}
		}
		return sizer{1 + 8, 0, 0}
	case reflect.Float32, reflect.Float64:
		return sizer{1 + uint(rtype.Size()), 0, 0}
	case reflect.Complex64, reflect.Complex128:
		return sizer{2 + uint(rtype.Size()), 0, 0}
	case reflect.String:
		val := value.String()
		if len(val) > 30 {
			return sizer{3 + uint(len(val)), 1, 0}
		}
		return sizer{1 + uint(len(val)), 1, 0}
	case reflect.Array:
		val := rtype.Len()
		small := sizer{1, 0, 0}
		if val > 30 {
			small.bytes += 2
		}
		for i := 0; i < val; i++ {
			small.add(enc.sizer(rtype.Elem(), value.Index(i)))
		}
		return small
	default:
		panic("unsupported box.sizer type " + rtype.String())
	}
}

func (enc Encoder) basic(box uint64, rtype reflect.Type, value reflect.Value, hint string) (template, error) {
	if enc.packed && value.IsZero() {
		return template{}, nil
	}
	fast := template{fast: rcopy{0, int64(rtype.Size()), ""}}
	var size Object
	switch rtype.Size() {
	case 1:
		size = ObjectBytes1
	case 2:
		size = ObjectBytes2
	case 4:
		size = ObjectBytes4
	case 8:
		size = ObjectBytes8
	}
	switch rtype.Kind() {
	case reflect.Bool:
		return fast, enc.object(box, ObjectBytes1, SchemaBoolean, hint)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
		if enc.packed {
			val := value.Uint()
			if val <= math.MaxUint8 {
				size = ObjectBytes1
				fast.fast.length = 1
			} else if val <= math.MaxUint16 {
				size = ObjectBytes2
				fast.fast.length = 2
			} else if val <= math.MaxUint32 {
				size = ObjectBytes4
				fast.fast.length = 4
			}
		}
		return fast, enc.object(box, size, SchemaNatural, hint)
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		if enc.packed {
			val := value.Int()
			if val <= math.MaxInt8 && val >= math.MinInt8 {
				size = ObjectBytes1
				fast.fast.length = 1
			} else if val <= math.MaxUint16 && val >= math.MinInt16 {
				size = ObjectBytes2
				fast.fast.length = 2
			} else if val <= math.MaxUint32 && val >= math.MinInt32 {
				size = ObjectBytes4
				fast.fast.length = 4
			}
		}
		return fast, enc.object(box, size, SchemaInteger, hint)
	case reflect.Float32:
		return fast, enc.object(box, ObjectBytes4, SchemaIEEE754, hint)
	case reflect.Float64:
		return fast, enc.object(box, ObjectBytes8, SchemaIEEE754, hint)
	case reflect.Complex64:
		if err := enc.object(2, ObjectRepeat, 0, hint); err != nil {
			return fast, err
		}
		return fast, enc.object(box, ObjectBytes4, SchemaIEEE754, hint)
	case reflect.Complex128:
		if err := enc.object(2, ObjectRepeat, 0, hint); err != nil {
			return fast, err
		}
		return fast, enc.object(box, ObjectBytes8, SchemaIEEE754, hint)
	case reflect.Array:
		size := rtype.Len()
		if size > math.MaxUint16 {
			return fast, fmt.Errorf("array size %d exceeds box maximum %d", size, math.MaxUint16)
		}
		if err := enc.object(uint64(size), ObjectRepeat, 0, hint); err != nil {
			return fast, err
		}
		specific, err := enc.basic(box, rtype.Elem(), reflect.Value{}, hint)
		if err != nil {
			return fast, err
		}
		if len(specific.then) == 0 {
			return fast, nil
		}
		for i := 0; i < size; i++ {
			if i > 0 {
				specific.slow = append(specific.slow, specific.fast)
			}
			for _, op := range specific.slow {
				specific.slow = append(specific.slow, op)
			}
			for _, op := range specific.then {
				specific.then = append(specific.then, op)
			}
		}
		return specific, nil
	case reflect.String:
		if enc.packed {
			if err := enc.object(uint64(value.Len()), ObjectRepeat, SchemaUnicode, hint); err != nil {
				return fast, err
			}
			fast.fast.string = value.String()
			return fast, enc.object(box, ObjectBytes1, SchemaNatural, hint)
		}
		if err := enc.object(box, ObjectMemory, 0, hint); err != nil {
			return fast, err
		}
		var bytes int64 = 8
		if enc.config&MemorySize4 != 0 {
			if enc.packed {
				if value.Len() <= math.MaxUint8 {
					bytes = 1
					if err := enc.object(box, ObjectBytes1, SchemaNatural, hint); err != nil {
						return fast, err
					}
				} else if value.Len() <= math.MaxUint16 {
					bytes = 2
					if err := enc.object(box, ObjectBytes2, SchemaNatural, hint); err != nil {
						return fast, err
					}
				} else {
					bytes = 4
					if err := enc.object(box, ObjectBytes4, SchemaNatural, hint); err != nil {
						return fast, err
					}
				}
			} else {
				bytes = 4
				if err := enc.object(box, ObjectBytes4, SchemaInteger, hint); err != nil {
					return fast, err
				}
			}
		} else {
			if err := enc.object(box, ObjectBytes8, SchemaInteger, hint); err != nil {
				return fast, err
			}
		}
		var specific template
		specific.slow = append(specific.slow, rcopy{-int64(value.Len()), -bytes, ""})
		specific.slow = append(specific.slow, rcopy{int64(value.Len()), -bytes, ""})
		specific.then = append(specific.then, after{offset: 0, handle: rtype})
		return specific, nil
	case reflect.Interface:
		if err := enc.object(2, ObjectStruct, SchemaDynamic, hint); err != nil {
			return fast, err
		}
		if err := enc.object(0, 0, 0, hint); err != nil {
			return fast, err
		}
		if err := enc.object(1, ObjectBytes8, 0, hint); err != nil {
			return fast, err
		}
		if err := enc.object(0, 0, 0, hint); err != nil {
			return fast, err
		}
		if err := enc.object(2, ObjectBytes8, 0, hint); err != nil {
			return fast, err
		}
		var specific template
		specific.fast = rcopy{0, -8, ""}                        // FIXME type pointer.
		specific.slow = append(specific.slow, rcopy{-0, 8, ""}) // FIXME negative memory.
		specific.then = append(specific.then, after{})
		specific.then = append(specific.then, after{})
		return specific, enc.end()
	case reflect.Slice:
		if err := enc.object(2, ObjectStruct, 0, hint); err != nil {
			return fast, err
		}
		if err := enc.object(0, 0, 0, hint); err != nil {
			return fast, err
		}
		if err := enc.object(1, ObjectBytes8, 0, hint); err != nil {
			return fast, err
		}
		if err := enc.object(2, ObjectBytes8, SchemaInteger, hint); err != nil {
			return fast, err
		}
		if err := enc.object(3, ObjectBytes8, SchemaInteger, hint); err != nil {
			return fast, err
		}
		var specific template
		specific.fast = rcopy{8, 8, ""}
		specific.slow = append(specific.slow, rcopy{-0, 8, ""}) // FIXME negative memory.
		specific.slow = append(specific.slow, rcopy{16, 8, ""}) // FIXME negative memory.
		specific.then = append(specific.then, after{})
		return specific, enc.end()
	case reflect.Map:
		if err := enc.object(2, ObjectStruct, SchemaMapping, hint); err != nil {
			return fast, err
		}
		if err := enc.object(0, 0, 0, hint); err != nil {
			return fast, err
		}
		if err := enc.object(1, ObjectBytes8, 0, hint); err != nil {
			return fast, err
		}
		var specific template
		specific.slow = append(specific.slow, rcopy{-0, 8, ""}) // FIXME negative memory.
		specific.then = append(specific.then, after{})
		return fast, enc.end()
	case reflect.Struct:
		if box > 1 {
			if err := enc.object(box, ObjectStruct, SchemaDefined, hint); err != nil {
				return fast, err
			}
		}
		var specific template
		var offset uintptr
		var sizing uintptr
		padding := func(reached uintptr) error {
			if enc.packed {
				return nil
			}
			if reached > offset+sizing {
				if err := enc.object(uint64(reached-(offset+sizing)), ObjectIgnore, SchemaUnknown, hint); err != nil {
					return err
				}
				if len(specific.slow) == 0 && specific.fast.length == 0 {
					specific.fast.offset = 0
					specific.fast.length = -int64(rtype.Size() - offset - sizing)
				} else {
					specific.slow = append(specific.slow, rcopy{0, -int64(rtype.Size() - offset - sizing), ""}) // FIXME negative memory.
				}
			}
			return nil
		}
		for i := 0; i < rtype.NumField(); i++ {
			field := rtype.Field(i)
			if err := padding(field.Offset); err != nil {
				return fast, err
			}
			offset = field.Offset
			sizing = field.Type.Size()
			box := uint64(i + 1)
			tag, ok := field.Tag.Lookup("box")
			if ok {
				if _, err := fmt.Sscanf(tag, "%d", &box); err != nil {
					return fast, fmt.Errorf("invalid box tag %q: %v", tag, err)
				}
			}
			elem := reflect.Value{}
			if value.IsValid() {
				elem = value.Field(i)
			}
			encoder, err := enc.basic(box, field.Type, elem, field.Name)
			if err != nil {
				return fast, err
			}
			if encoder.fast == (rcopy{}) && len(encoder.slow) == 0 {
				continue
			}
			encoder.fast.offset += int64(field.Offset)
			for _, slow := range encoder.slow {
				slow.offset += int64(field.Offset)
			}
			if enc.packed {
				if len(encoder.slow) == 0 && len(specific.slow) == 0 && specific.fast.length == 0 {
					specific.fast.offset = int64(field.Offset) + encoder.fast.offset
					specific.fast.length = encoder.fast.length
					specific.fast.string = encoder.fast.string
				} else {
					specific.slow = append(specific.slow, encoder.fast)
					specific.slow = append(specific.slow, encoder.slow...)
					specific.then = append(specific.then, encoder.then...)
				}
			} else {
				if len(encoder.slow) == 0 && len(specific.slow) == 0 && specific.fast.length == 0 && encoder.fast.length != 0 {
					specific.fast.length = int64(field.Offset + field.Type.Size())
				} else {
					specific.slow = append(specific.slow, encoder.slow...)
					specific.then = append(specific.then, encoder.then...)
				}
			}
		}
		if err := padding(rtype.Size()); err != nil {
			return fast, err
		}
		if box > 1 {
			if err := enc.end(); err != nil {
				return fast, err
			}
		}
		return specific, nil
	case reflect.Pointer, reflect.UnsafePointer:
		if err := enc.object(0, 0, 0, hint); err != nil {
			return fast, err
		}
		if rtype.Size() == 8 {
			return fast, enc.object(box, ObjectBytes8, 0, hint)
		}
		var specific template
		specific.slow = append(specific.slow, rcopy{0, 8, ""}) // FIXME negative memory.
		specific.then = append(specific.then, after{offset: 0, handle: rtype.Elem()})
		return fast, enc.object(box, ObjectBytes4, 0, hint)
	case reflect.Chan:
		var specific template
		specific.slow = append(specific.slow, rcopy{0, 8, ""}) // FIXME negative memory.
		specific.then = append(specific.then, after{offset: 0, handle: rtype.Elem()})
		return fast, enc.object(box, ObjectBytes8, SchemaChannel, hint)
	case reflect.Func:
		if err := enc.object(2, ObjectStruct, SchemaProgram, hint); err != nil {
			return fast, err
		}
		if err := enc.object(0, 0, 0, hint); err != nil {
			return fast, err
		}
		if err := enc.object(1, ObjectBytes8, 0, hint); err != nil {
			return fast, err
		}
		var specific template
		specific.slow = append(specific.slow, rcopy{0, 8, ""}) // FIXME negative memory.
		specific.then = append(specific.then, after{offset: 0, handle: reflect.TypeOf("")})
		return specific, enc.end()
	}
	return fast, fmt.Errorf("unsupported type %v", rtype)
}

func (enc Encoder) value(specific template, value reflect.Value) error {
	ptr := value.Addr().UnsafePointer()
	var memory int
	if err := specific.fast.copy(enc.w, ptr, &memory); err != nil {
		return xray.New(err)
	}
	for i := range specific.slow {
		if err := specific.slow[i].copy(enc.w, ptr, &memory); err != nil {
			return xray.New(err)
		}
	}
	if len(specific.then) == 0 {
		return nil
	}
	if enc.config&BinaryEndian != 0 {
		if enc.config&MemorySize4 != 0 {
			var buf [2]byte
			binary.BigEndian.PutUint16(buf[:], uint16(memory))
			if _, err := enc.w.Write(buf[:]); err != nil {
				return err
			}
		} else {
			var buf [4]byte
			binary.BigEndian.PutUint32(buf[:], uint32(memory))
			if _, err := enc.w.Write(buf[:]); err != nil {
				return err
			}
		}
	} else {
		if enc.config&MemorySize4 != 0 {
			var buf [2]byte
			binary.LittleEndian.PutUint16(buf[:], uint16(memory))
			if _, err := enc.w.Write(buf[:]); err != nil {
				return err
			}
		} else {
			var buf [4]byte
			binary.LittleEndian.PutUint32(buf[:], uint32(memory))
			if _, err := enc.w.Write(buf[:]); err != nil {
				return err
			}
		}
	}
	for _, after := range specific.then {
		elem := reflect.NewAt(after.handle, unsafe.Pointer(uintptr(ptr)+after.offset)).Elem()
		switch after.handle.Kind() {
		case reflect.String:
			if err := enc.object(uint64(len(elem.String())), ObjectRepeat, SchemaUnicode, ""); err != nil {
				return err
			}
			if err := enc.object(0, ObjectBytes1, SchemaUnknown, ""); err != nil {
				return err
			}
			if _, err := enc.w.Write([]byte{0}); err != nil {
				return err
			}
			fmt.Fprint(enc.w, elem.String())
		}
	}
	return nil
}
