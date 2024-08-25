package box

import (
	"encoding/binary"
	"errors"
	"reflect"
	"strconv"
)

func (d *Decoder) slow(binary Binary, object []byte, rvalue reflect.Value, xvalue []byte) (offset int, n int, err error) {
	for i := 0; i < len(object); i++ {
		size := Object(object[i])
		args := int(size & 0b00011111)
		switch size & sizingMask {
		case ObjectRepeat:
			return offset, i, errors.New("not supported")
		case ObjectMemory:
			return offset, i, errors.New("not supported")
		case ObjectBytes1:
			if err := d.bytes(binary, rvalue, args, xvalue[offset:offset+1]); err != nil {
				return 0, 0, err
			}
			offset += 1
		case ObjectBytes2:
			if err := d.bytes(binary, rvalue, args, xvalue[offset:offset+2]); err != nil {
				return 0, 0, err
			}
			offset += 2
		case ObjectBytes4:
			if err := d.bytes(binary, rvalue, args, xvalue[offset:offset+4]); err != nil {
				return 0, 0, err
			}
			offset += 4
		case ObjectBytes8:
			if err := d.bytes(binary, rvalue, args, xvalue[offset:offset+8]); err != nil {
				return 0, 0, err
			}
			offset += 8
		case ObjectStruct:
			field, ok := d.box(rvalue, args)
			if !ok {
				skipping := 1
				for ; i < len(object); i++ {
					if Object(object[i])&sizingMask == ObjectStruct {
						skipping++
						continue
					}
					if Object(object[i])&sizingMask == ObjectIgnore {
						skipping--
						if skipping == 0 {
							break
						}
					}
				}
				if skipping != 0 {
					return offset, i, errors.New("box: invalid object schema")
				}
			} else {
				length, count, err := d.slow(binary, object[i+1:], field, xvalue[offset:])
				if err != nil {
					return offset, i, err
				}
				offset += length
				i += count
			}
		case ObjectIgnore:
			if args == 0 {
				return offset, i, nil
			}
			offset += args
		}
	}
	return offset, len(object), nil
}

func (d *Decoder) box(rvalue reflect.Value, n int) (reflect.Value, bool) {
	rtype := rvalue.Type()
	if n <= rtype.NumField() {
		field := rtype.Field(n)
		_, ok := field.Tag.Lookup("box")
		if !ok {
			return rvalue.Field(n), true
		}
	}
	for i := range rtype.NumField() {
		field := rtype.Field(i)
		b, ok := field.Tag.Lookup("box")
		if !ok {
			return rvalue.Field(i), true
		}
		box, err := strconv.Atoi(b)
		if err != nil {
			continue
		}
		if box == n {
			return rvalue.Field(i), true
		}
	}
	return reflect.Value{}, false
}

func (d *Decoder) bytes(header Binary, rvalue reflect.Value, args int, buffer []byte) error {
	if rvalue.Kind() == reflect.Struct {
		field, ok := d.box(rvalue, args)
		if !ok {
			return nil
		}
		rvalue = field
	}

	var endian binary.ByteOrder = binary.LittleEndian
	if header&BinaryEndian != 0 {
		endian = binary.BigEndian
	}
	switch rvalue.Kind() {
	case reflect.Bool:
		for _, b := range buffer {
			if b != 0 {
				rvalue.SetBool(true)
				return nil
			}
		}
		rvalue.SetBool(false)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch len(buffer) {
		case 1:
			var u8 = buffer[0]
			if u8&0b1<<7 != 0 {
				u8 = u8 & 0b01111111
				rvalue.SetInt(int64(-u8))
			} else {
				rvalue.SetInt(int64(u8))
			}
		case 2:
			var u16 = endian.Uint16(buffer)
			if u16&0b1<<15 != 0 {
				u16 = u16 & 0b0111111111111111
				rvalue.SetInt(int64(-u16))
			} else {
				rvalue.SetInt(int64(u16))
			}
		case 4:
			var u32 = endian.Uint32(buffer)
			if u32&0b1<<31 != 0 {
				u32 = u32 & 0b01111111111111111111111111111111
				rvalue.SetInt(int64(-u32))
			} else {
				rvalue.SetInt(int64(u32))
			}
		case 8:
			var u64 = endian.Uint64(buffer)
			if u64&0b1<<63 != 0 {
				u64 = u64 & 0b011
				rvalue.SetInt(int64(-u64))
			} else {
				rvalue.SetInt(int64(u64))
			}
		default:
			return errors.New("box: unsupported integer size " + strconv.Itoa(len(buffer)))
		}
	default:
		return errors.New("box: unsupported type " + rvalue.Kind().String())
	}
	return nil
}
