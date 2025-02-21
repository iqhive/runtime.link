package ffi

import (
	"encoding/binary"
	"io"
	"math"
	"reflect"
	"unsafe"
)

func sizeOf(rtype reflect.Type) uintptr {
	switch rtype.Kind() {
	case reflect.String, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func, reflect.Ptr,
		reflect.UnsafePointer, reflect.Interface:
		return unsafe.Sizeof(uintptr(0))
	case reflect.Array:
		return uintptr(rtype.Len()) * sizeOf(rtype.Elem())
	case reflect.Struct:
		var size uintptr
		for i := range rtype.NumField() {
			size += sizeOf(rtype.Field(i).Type)
		}
		return size
	default:
		return uintptr(rtype.Size())
	}
}

func writeValue(dst io.WriterAt, off int64, value reflect.Value) {
	panic("not implemented")
}

func writeBytes(dst reflect.Value, off int64, value io.ReaderAt) {
	panic("not implemented")
}

func read(buf []byte, i int64, value reflect.Value) int64 {
	switch value.Kind() {
	case reflect.String, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func, reflect.Ptr,
		reflect.UnsafePointer, reflect.Interface:
		binary.LittleEndian.PutUint64(buf[i:], uint64(new_ref(value)))
		return 8
	case reflect.Int8:
		buf[i] = byte(value.Int())
		return 1
	case reflect.Int16:
		binary.LittleEndian.PutUint16(buf[i:], uint16(value.Int()))
		return 2
	case reflect.Int32:
		binary.LittleEndian.PutUint32(buf[i:], uint32(value.Int()))
		return 4
	case reflect.Int64:
		binary.LittleEndian.PutUint64(buf[i:], uint64(value.Int()))
		return 8
	case reflect.Int:
		if value.Type().Size() == 4 {
			binary.LittleEndian.PutUint32(buf[i:], uint32(value.Int()))
			return 4
		}
		binary.LittleEndian.PutUint64(buf[i:], uint64(value.Int()))
		return 8
	case reflect.Uint8:
		buf[i] = byte(value.Uint())
		return 1
	case reflect.Uint16:
		binary.LittleEndian.PutUint16(buf[i:], uint16(value.Uint()))
		return 2
	case reflect.Uint32:
		binary.LittleEndian.PutUint32(buf[i:], uint32(value.Uint()))
		return 4
	case reflect.Uint64:
		binary.LittleEndian.PutUint64(buf[i:], value.Uint())
		return 8
	case reflect.Uintptr, reflect.Uint:
		if value.Type().Size() == 4 {
			binary.LittleEndian.PutUint32(buf[i:], uint32(value.Uint()))
			return 4
		}
		binary.LittleEndian.PutUint64(buf[i:], value.Uint())
		return 8
	case reflect.Float32:
		binary.LittleEndian.PutUint32(buf[i:], math.Float32bits(float32(value.Float())))
		return 4
	case reflect.Float64:
		binary.LittleEndian.PutUint64(buf[i:], math.Float64bits(value.Float()))
		return 8
	case reflect.Complex64:
		binary.LittleEndian.PutUint32(buf[i:], math.Float32bits(float32(real(value.Complex()))))
		binary.LittleEndian.PutUint32(buf[i+4:], math.Float32bits(float32(imag(value.Complex()))))
		return 16
	case reflect.Complex128:
		binary.LittleEndian.PutUint64(buf[i:], math.Float64bits(real(value.Complex())))
		binary.LittleEndian.PutUint64(buf[i+8:], math.Float64bits(imag(value.Complex())))
		return 16
	case reflect.Array:
		var n int64
		for j := range value.Len() {
			n += read(buf, i+n, value.Index(j))
		}
		return n
	case reflect.Struct:
		var n int64
		for j := range value.NumField() {
			n += read(buf, i+n, value.Field(j))
		}
		return n
	default:
		panic("unsupported type: " + value.Type().String())
	}
}

type sliceReader struct {
	slice    reflect.Value
	elemSize uintptr
}

func newSliceReader(slice reflect.Value) *sliceReader {
	return &sliceReader{slice: slice, elemSize: sizeOf(slice.Type().Elem())}
}

func (r sliceReader) ReadAt(buf []byte, off int64) (n int, err error) {
	if off%int64(r.elemSize) != 0 || len(buf)%int(r.elemSize) != 0 {
		panic("unaligned read")
	}
	count := len(buf) / int(r.elemSize)
	index := int(off / int64(r.elemSize))
	for i := range count {
		elem := r.slice.Index(index + i)
		n = int(read(buf, int64(n), elem))
	}
	return count, nil
}

type pointerBytes struct {
	pointer reflect.Value
}

func newPointerBytes(pointer reflect.Value) *pointerBytes {
	return &pointerBytes{pointer: pointer}
}

func (b pointerBytes) ReadAt(buf []byte, off int64) (n int, err error) {
	return int(read(buf, off, b.pointer)), nil
}
