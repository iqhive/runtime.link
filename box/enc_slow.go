package box

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"unsafe"
)

// new adds a pointer to the ptrs stack, returns true if the pointer was just added
// and false if the pointer ha already been added.
func (enc *Encoder) new(value reflect.Value) bool {
	ptr := value.UnsafePointer()
	if ref, ok := enc.ref[ptr]; ok {
		if ref == 0 {
			ref = enc.ram
			enc.ref[ptr] = ref
			enc.ptr = append(enc.ptr, ref)
			return true
		}
		enc.ptr = append(enc.ptr, ref)
		return false
	}
	if enc.ref == nil {
		enc.ref = make(map[unsafe.Pointer]uintptr)
	}
	enc.ptr = append(enc.ptr, enc.ram)
	enc.ref[ptr] = enc.ram
	return true
}

func (enc *Encoder) pop() error {
	enc.ram += 8
	buf := [8]byte{}
	ptr := enc.ptr[len(enc.ptr)-1]
	enc.ptr = enc.ptr[:len(enc.ptr)-1]
	if enc.binary&BinaryEndian != 0 {
		binary.BigEndian.PutUint64(buf[:], uint64(ptr))
	} else {
		binary.LittleEndian.PutUint64(buf[:], uint64(ptr))
	}
	if _, err := enc.w.Write(buf[:]); err != nil {
		return err
	}
	return nil
}

func (enc *Encoder) pointers(value reflect.Value) error {
	switch value.Kind() {
	case reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if err := enc.pointers(value.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		if value.Len() == 0 {
			enc.ptr = append(enc.ptr, 0)
			return nil
		}
		keys := value.MapKeys()
		for i := 0; i < len(keys); i++ {
			if err := enc.pointers(keys[i]); err != nil {
				return err
			}
			if err := enc.pointers(value.MapIndex(keys[i])); err != nil {
				return err
			}
		}
		if enc.new(value) {
			for _, key := range keys {
				if err := enc.x(key); err != nil {
					return err
				}
				if err := enc.x(value.MapIndex(key)); err != nil {
					return err
				}
			}
		}
	case reflect.Slice:
		length := value.Len()
		if length == 0 {
			enc.ptr = append(enc.ptr, 0, 0, 0)
			return nil
		}
		isnew := enc.new(value)
		enc.ptr = append(enc.ptr, uintptr(length))
		enc.ptr = append(enc.ptr, uintptr(value.Cap()))
		if isnew {
			for i := 0; i < length; i++ {
				if err := enc.pointers(value.Index(i)); err != nil {
					return err
				}
			}
			for i := 0; i < length; i++ {
				if err := enc.x(value.Index(i)); err != nil {
					return err
				}
			}
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if err := enc.pointers(value.Field(i)); err != nil {
				return err
			}
		}
	case reflect.Pointer:
		if value.IsNil() {
			enc.ptr = append(enc.ptr, 0)
			return nil
		}
		if enc.new(value) {
			if err := enc.x(value.Elem()); err != nil {
				return err
			}
		}
	case reflect.String:
		if value.Len() == 0 {
			enc.ptr = append(enc.ptr, 0)
			enc.ptr = append(enc.ptr, 0)
			return nil
		}
		enc.ptr = append(enc.ptr, uintptr(value.Len()))
		isNew := enc.new(value)
		if isNew {
			_, err := enc.w.WriteString(value.String())
			return err
		}
	}
	return nil
}

func (enc *Encoder) x(value reflect.Value) error {
	if enc.packed() && value.IsZero() {
		return nil
	}
	switch value.Kind() {
	case reflect.Bool:
		enc.ram++
		if value.Bool() {
			return enc.w.WriteByte(1)
		}
		return enc.w.WriteByte(0)
	case reflect.Int8:
		var val = int8(value.Int())
		enc.ram++
		return enc.w.WriteByte(*(*byte)(unsafe.Pointer(&val)))
	case reflect.Uint8:
		enc.ram++
		return enc.w.WriteByte(value.Interface().(byte))
	case reflect.Int16:
		var val = int16(value.Int())
		enc.ram += 2
		var buf [2]byte
		if enc.binary&BinaryEndian != 0 {
			binary.BigEndian.PutUint16(buf[:], *(*uint16)(unsafe.Pointer(&val)))
		} else {
			binary.LittleEndian.PutUint16(buf[:], *(*uint16)(unsafe.Pointer(&val)))
		}
		_, err := enc.w.Write(buf[:])
		return err
	case reflect.Uint16:
		enc.ram += 2
		var buf [2]byte
		if enc.binary&BinaryEndian != 0 {
			binary.BigEndian.PutUint16(buf[:], uint16(value.Uint()))
		} else {
			binary.LittleEndian.PutUint16(buf[:], uint16(value.Uint()))
		}
		_, err := enc.w.Write(buf[:])
		return err
	case reflect.Int32:
		var val = int32(value.Int())
		enc.ram += 4
		var buf [4]byte
		if enc.binary&BinaryEndian != 0 {
			binary.BigEndian.PutUint32(buf[:], *(*uint32)(unsafe.Pointer(&val)))
		} else {
			binary.LittleEndian.PutUint32(buf[:], *(*uint32)(unsafe.Pointer(&val)))
		}
		_, err := enc.w.Write(buf[:])
		return err
	case reflect.Uint32:
		enc.ram += 4
		var buf [4]byte
		if enc.binary&BinaryEndian != 0 {
			binary.BigEndian.PutUint32(buf[:], uint32(value.Uint()))
		} else {
			binary.LittleEndian.PutUint32(buf[:], uint32(value.Uint()))
		}
		_, err := enc.w.Write(buf[:])
		return err
	case reflect.Int64:
		var val = int64(value.Int())
		enc.ram += 8
		var buf [8]byte
		if enc.binary&BinaryEndian != 0 {
			binary.BigEndian.PutUint64(buf[:], *(*uint64)(unsafe.Pointer(&val)))
		} else {
			binary.LittleEndian.PutUint64(buf[:], *(*uint64)(unsafe.Pointer(&val)))
		}
		_, err := enc.w.Write(buf[:])
		return err
	case reflect.Uint64:
		enc.ram += 8
		var buf [8]byte
		if enc.binary&BinaryEndian != 0 {
			binary.BigEndian.PutUint64(buf[:], value.Uint())
		} else {
			binary.LittleEndian.PutUint64(buf[:], value.Uint())
		}
		_, err := enc.w.Write(buf[:])
		return err
	case reflect.Int:
		if value.Type().Size() == 8 {
			enc.ram += 8
			return enc.x(reflect.ValueOf(value.Int()))
		}
		enc.ram += 4
		return enc.x(reflect.ValueOf(int32(value.Int())))
	case reflect.Uint, reflect.Uintptr:
		if value.Type().Size() == 8 {
			enc.ram += 8
			return enc.x(reflect.ValueOf(value.Uint()))
		}
		enc.ram += 4
		return enc.x(reflect.ValueOf(uint32(value.Uint())))
	case reflect.Float32:
		enc.ram += 4
		var buf [4]byte
		if enc.binary&BinaryEndian != 0 {
			binary.BigEndian.PutUint32(buf[:], math.Float32bits(float32(value.Float())))
		} else {
			binary.LittleEndian.PutUint32(buf[:], math.Float32bits(float32(value.Float())))
		}
		_, err := enc.w.Write(buf[:])
		return err
	case reflect.Float64:
		enc.ram += 8
		var buf [8]byte
		if enc.binary&BinaryEndian != 0 {
			binary.BigEndian.PutUint64(buf[:], math.Float64bits(value.Float()))
		} else {
			binary.LittleEndian.PutUint64(buf[:], math.Float64bits(value.Float()))
		}
		_, err := enc.w.Write(buf[:])
		return err
	case reflect.Complex64:
		enc.ram += 8
		var buf [8]byte
		if enc.binary&BinaryEndian != 0 {
			binary.BigEndian.PutUint32(buf[:4], math.Float32bits(float32(real(value.Complex()))))
			binary.BigEndian.PutUint32(buf[4:], math.Float32bits(float32(imag(value.Complex()))))
		} else {
			binary.LittleEndian.PutUint32(buf[:4], math.Float32bits(float32(real(value.Complex()))))
			binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(float32(imag(value.Complex()))))
		}
		_, err := enc.w.Write(buf[:])
		if err != nil {
			return err
		}
		return nil
	case reflect.Complex128:
		enc.ram += 16
		var buf [16]byte
		if enc.binary&BinaryEndian != 0 {
			binary.BigEndian.PutUint64(buf[:8], math.Float64bits(real(value.Complex())))
			binary.BigEndian.PutUint64(buf[8:], math.Float64bits(imag(value.Complex())))
		} else {
			binary.LittleEndian.PutUint64(buf[:8], math.Float64bits(real(value.Complex())))
			binary.LittleEndian.PutUint64(buf[8:], math.Float64bits(imag(value.Complex())))
		}
		_, err := enc.w.Write(buf[:])
		if err != nil {
			return err
		}
		return nil
	case reflect.String:
		if err := enc.pop(); err != nil {
			return err
		}
		return enc.pop()
	case reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if err := enc.x(value.Index(i)); err != nil {
				return err
			}
		}
		return nil
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if err := enc.x(value.Field(i)); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		return enc.pop()
	case reflect.Slice:
		if err := enc.pop(); err != nil {
			return err
		}
		if err := enc.pop(); err != nil {
			return err
		}
		return enc.pop()
	}
	return fmt.Errorf("unsupported type %s", value.Type())
}
