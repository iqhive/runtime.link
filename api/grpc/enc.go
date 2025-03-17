package grpc

import (
	"errors"
	"io"
	"math"
	"reflect"
	"unsafe"

	_ "unsafe"
)

// Encoder for the GRPC Protocol Buffers format
type Encoder struct {
	w io.Writer
}

//go:linkname write runtime.link/api/grpc.write_impl
//go:noescape
func write(w io.Writer, buf []byte) (int, error)

func write_string(w io.Writer, s string) (int, error) {
	return write(w, unsafe.Slice(unsafe.StringData(s), len(s)))
}

func write_impl(w io.Writer, buf []byte) (int, error) {
	return w.Write(buf)
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

func (e *Encoder) Encode(v any) error {
	if err := e.encode(1, reflect.ValueOf(v)); err != nil {
		return err
	}
	return nil
}

func sizeof(n wireNumber, rvalue reflect.Value) (int, int) {
	rtype := rvalue.Type()
	switch rtype.Kind() {
	case reflect.Bool:
		return sizeTag(n), 1
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return sizeTag(n), sizeVarint(encodeInt(rvalue.Int()))
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		return sizeTag(n), sizeVarint(rvalue.Uint())
	case reflect.Float32:
		return sizeTag(n), 4
	case reflect.Float64:
		return sizeTag(n), 8
	case reflect.String:
		return sizeTag(n) + sizeVarint(uint64(len(rvalue.String()))), len(rvalue.String())
	case reflect.Array:
		var size int
		for i := 0; i < rvalue.Len(); i++ {
			head, body := sizeof(wireNumber(i)+1, rvalue.Index(i))
			size += head + body
		}
		return sizeTag(n) + sizeVarint(uint64(size)), size
	case reflect.Slice:
		var size int
		for i := 0; i < rvalue.Len(); i++ {
			head, body := sizeof(n, rvalue.Index(i))
			size += head + body
		}
		return 0, size
	case reflect.Map:
		return sizeof_map_noescape(n, rvalue)
	case reflect.Interface, reflect.Pointer:
		if rvalue.IsNil() {
			return 0, 0
		}
		return sizeof(n, rvalue.Elem())
	case reflect.Struct:
		var size int
		for i := range rvalue.NumField() {
			head, body := sizeof(wireNumber(i)+1, rvalue.Field(i))
			size += head + body
		}
		return sizeTag(n) + sizeVarint(uint64(size)), size
	}
	return -1, 0
}

func (e *Encoder) encode(n wireNumber, rvalue reflect.Value) error {
	var stack [16]byte
	var slice = stack[0:0:cap(stack)]
	rtype := rvalue.Type()
	switch rtype.Kind() {
	case reflect.Bool:
		slice = appendTag(slice[:], n, typeVarint)
		slice = appendVarint(slice[:], encodeBool(rvalue.Bool()))
		_, err := write(e.w, slice)
		return err
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		slice = appendTag(slice[:], n, typeVarint)
		slice = appendVarint(slice[:], encodeInt(rvalue.Int()))
		_, err := write(e.w, slice)
		return err
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		slice = appendTag(slice[:], n, typeVarint)
		slice = appendVarint(slice[:], rvalue.Uint())
		_, err := write(e.w, slice)
		return err
	case reflect.Float32:
		slice = appendTag(slice[:], n, typeFixed32)
		slice = appendFixed32(slice[:], math.Float32bits(float32(rvalue.Float())))
		_, err := write(e.w, slice)
		return err
	case reflect.Float64:
		slice = appendTag(slice[:], n, typeFixed64)
		slice = appendFixed64(slice[:], math.Float64bits(rvalue.Float()))
		_, err := write(e.w, slice)
		return err
	case reflect.String:
		slice = appendTag(slice[:], n, typeBytes)
		slice = appendVarint(slice[:], uint64(len(rvalue.String())))
		if _, err := write(e.w, slice); err != nil {
			return err
		}
		_, err := write_string(e.w, rvalue.String())
		return err
	case reflect.Array:
		slice = appendTag(slice[:], n, typeBytes)
		_, size := sizeof(n, rvalue)
		slice = appendVarint(slice[:], uint64(size))
		if _, err := write(e.w, slice); err != nil {
			return err
		}
		for i := 0; i < rvalue.Len(); i++ {
			if err := e.encode(wireNumber(i)+1, rvalue.Index(i)); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice:
		for i := 0; i < rvalue.Len(); i++ {
			if err := e.encode(n, rvalue.Index(i)); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		return encode_map_noescape(e, n, rvalue)
	case reflect.Interface, reflect.Pointer:
		if rvalue.IsNil() {
			return nil
		}
		return e.encode(n, rvalue.Elem())
	case reflect.Struct:
		slice = appendTag(slice[:], n, typeBytes)
		_, size := sizeof(n, rvalue)
		slice = appendVarint(slice[:], uint64(size))
		if _, err := write(e.w, slice); err != nil {
			return err
		}
		for i := 0; i < rvalue.NumField(); i++ {
			if err := e.encode(wireNumber(i)+1, rvalue.Field(i)); err != nil {
				return err
			}
		}
		return nil
	}
	return errors.New("unsupported type")
}

//go:linkname sizeof_map_noescape runtime.link/api/grpc.sizeof_map
//go:noescape
func sizeof_map_noescape(n wireNumber, rvalue reflect.Value) (int, int)

//go:linkname encode_map_noescape runtime.link/api/grpc.encode_map
//go:noescape
func encode_map_noescape(e *Encoder, n wireNumber, rvalue reflect.Value) error

//go:linkname reflect_new_at_noescape runtime.link/api/grpc.reflect_new_at
//go:noescape
func reflect_new_at_noescape(t reflect.Type, p unsafe.Pointer) reflect.Value

func reflect_new_at(t reflect.Type, p unsafe.Pointer) reflect.Value {
	return reflect.NewAt(t, p)
}

func sizeof_map(n wireNumber, rvalue reflect.Value) (int, int) {
	var size int
	iter := rvalue.MapRange()
	var keyBuf, valBuf [3]uintptr
	var key, val reflect.Value
	if rvalue.Type().Key().Size() < 24 {
		key = reflect_new_at_noescape(rvalue.Type().Key(), unsafe.Pointer(&keyBuf)).Elem()
	} else {
		key = reflect.New(rvalue.Type().Key()).Elem()
	}
	if rvalue.Type().Elem().Size() < 24 {
		val = reflect_new_at_noescape(rvalue.Type().Elem(), unsafe.Pointer(&valBuf)).Elem()
	} else {
		val = reflect.New(rvalue.Type().Elem()).Elem()
	}
	for iter.Next() {
		key.SetIterKey(iter)
		val.SetIterValue(iter)
		keyHead, keyBody := sizeof(1, key)
		valHead, valBody := sizeof(2, val)
		size += keyHead + keyBody + valHead + valBody
	}
	return sizeTag(n) + sizeVarint(uint64(size)), size
}

func encode_map(e *Encoder, n wireNumber, rvalue reflect.Value) error {
	var stack [16]byte
	var slice = stack[0:0:cap(stack)]
	_, size := sizeof_map(n, rvalue)
	size /= rvalue.Len()
	iter := rvalue.MapRange()
	var keyBuf, valBuf [3]uintptr
	var key, val reflect.Value
	if rvalue.Type().Key().Size() < 24 {
		key = reflect_new_at_noescape(rvalue.Type().Key(), unsafe.Pointer(&keyBuf)).Elem()
	} else {
		key = reflect.New(rvalue.Type().Key()).Elem()
	}
	if rvalue.Type().Elem().Size() < 24 {
		val = reflect_new_at_noescape(rvalue.Type().Elem(), unsafe.Pointer(&valBuf)).Elem()
	} else {
		val = reflect.New(rvalue.Type().Elem()).Elem()
	}
	for iter.Next() {
		key.SetIterKey(iter)
		val.SetIterValue(iter)
		slice = stack[0:0:cap(stack)]
		slice = appendTag(slice[:], n, typeBytes)
		slice = appendVarint(slice[:], uint64(size))
		if _, err := write(e.w, slice); err != nil {
			return err
		}
		if err := e.encode(1, key); err != nil {
			return err
		}
		if err := e.encode(2, val); err != nil {
			return err
		}
	}
	return nil
}
