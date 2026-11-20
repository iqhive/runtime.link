// Package grpc implements Protocol Buffer codecs.
//
// A future gRPC linker can use [runtime.link/api.Function.Args] for named
// request fields when the API implements [runtime.link/api.WithSource].
package grpc

import (
	"errors"
	"io"
	"math"
	"reflect"
	"unsafe"

	_ "unsafe"
)

// Encoder for the GRPC Protocol Buffers format. Only exported struct fields
// are encoded; field numbers are their one-based positions in the struct,
// including positions occupied by unexported fields.
type Encoder struct {
	w io.Writer
}

//go:linkname write github.com/iqhive/runtime.link/api/grpc.write_impl
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

// maxDepth is how many values deep a value may nest. It keeps the recursion on
// a value that points back at itself off the end of the stack, which no caller
// could recover from.
const maxDepth = 10000

var (
	errUnsupported = errors.New("unsupported type")
	errTooDeep     = errors.New("exceeded max depth")
)

// Encode writes v as field 1 of a message. A value that holds a type with no
// Protocol Buffers form (a func, chan, complex number or uintptr), or that
// nests more than 10000 values deep as one that points back at itself does, is
// refused with nothing written.
func (e *Encoder) Encode(v any) error {
	if v == nil {
		return nil
	}
	rvalue := reflect.ValueOf(v)
	// Measuring walks the whole value, so a value that cannot be encoded is
	// refused here, before any of it is on the wire: half a message would
	// corrupt the stream for whoever reads it.
	if _, _, err := sizeof(1, rvalue, 0, false); err != nil {
		return err
	}
	return e.encode(1, rvalue, 0, false)
}

// sizeof returns the size of the tag and length that precede rvalue as field
// n, and the size of what follows them. The value is nested inside depth
// others. An empty bytes value still needs a field when it is a repeated element.
func sizeof(n wireNumber, rvalue reflect.Value, depth int, repeated bool) (head, body int, err error) {
	if depth > maxDepth {
		return 0, 0, errTooDeep
	}
	rtype := rvalue.Type()
	switch rtype.Kind() {
	case reflect.Bool:
		return sizeTag(n), 1, nil
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return sizeTag(n), sizeVarint(encodeInt(rvalue.Int())), nil
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		return sizeTag(n), sizeVarint(rvalue.Uint()), nil
	case reflect.Float32:
		return sizeTag(n), 4, nil
	case reflect.Float64:
		return sizeTag(n), 8, nil
	case reflect.String:
		return sizeTag(n) + sizeVarint(uint64(len(rvalue.String()))), len(rvalue.String()), nil
	case reflect.Array:
		var size int
		for i := 0; i < rvalue.Len(); i++ {
			head, body, err := sizeof(wireNumber(i)+1, rvalue.Index(i), depth+1, false)
			if err != nil {
				return 0, 0, err
			}
			size += head + body
		}
		return sizeTag(n) + sizeVarint(uint64(size)), size, nil
	case reflect.Slice:
		if isBytes(rtype) && (repeated || rvalue.Len() > 0) {
			return sizeTag(n) + sizeVarint(uint64(rvalue.Len())), rvalue.Len(), nil
		}
		var size int
		for i := 0; i < rvalue.Len(); i++ {
			head, body, err := sizeof(n, rvalue.Index(i), depth+1, true)
			if err != nil {
				return 0, 0, err
			}
			size += head + body
		}
		return 0, size, nil
	case reflect.Map:
		return sizeof_map_noescape(n, rvalue, depth)
	case reflect.Interface, reflect.Pointer:
		if rvalue.IsNil() {
			return 0, 0, nil
		}
		return sizeof(n, rvalue.Elem(), depth+1, repeated)
	case reflect.Struct:
		var size int
		for i := range rvalue.NumField() {
			if !rtype.Field(i).IsExported() {
				continue
			}
			head, body, err := sizeof(wireNumber(i)+1, rvalue.Field(i), depth+1, false)
			if err != nil {
				return 0, 0, err
			}
			size += head + body
		}
		return sizeTag(n) + sizeVarint(uint64(size)), size, nil
	}
	return 0, 0, errUnsupported
}

func (e *Encoder) encode(n wireNumber, rvalue reflect.Value, depth int, repeated bool) error {
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
		_, size, err := sizeof(n, rvalue, depth, false)
		if err != nil {
			return err
		}
		slice = appendVarint(slice[:], uint64(size))
		if _, err := write(e.w, slice); err != nil {
			return err
		}
		for i := 0; i < rvalue.Len(); i++ {
			if err := e.encode(wireNumber(i)+1, rvalue.Index(i), depth+1, false); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice:
		if isBytes(rtype) && (repeated || rvalue.Len() > 0) {
			slice = appendTag(slice[:], n, typeBytes)
			slice = appendVarint(slice[:], uint64(rvalue.Len()))
			if _, err := write(e.w, slice); err != nil {
				return err
			}
			_, err := write(e.w, rvalue.Bytes())
			return err
		}
		for i := 0; i < rvalue.Len(); i++ {
			if err := e.encode(n, rvalue.Index(i), depth+1, true); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		return encode_map_noescape(e, n, rvalue, depth)
	case reflect.Interface, reflect.Pointer:
		if rvalue.IsNil() {
			return nil
		}
		return e.encode(n, rvalue.Elem(), depth+1, repeated)
	case reflect.Struct:
		slice = appendTag(slice[:], n, typeBytes)
		_, size, err := sizeof(n, rvalue, depth, false)
		if err != nil {
			return err
		}
		slice = appendVarint(slice[:], uint64(size))
		if _, err := write(e.w, slice); err != nil {
			return err
		}
		for i := 0; i < rvalue.NumField(); i++ {
			if !rtype.Field(i).IsExported() {
				continue
			}
			if err := e.encode(wireNumber(i)+1, rvalue.Field(i), depth+1, false); err != nil {
				return err
			}
		}
		return nil
	}
	return errUnsupported
}

// isBytes reports whether a slice is the protobuf bytes type: a single length
// delimited field, where any other slice is one field for each element.
func isBytes(rtype reflect.Type) bool {
	return rtype.Elem().Kind() == reflect.Uint8
}

//go:linkname sizeof_map_noescape github.com/iqhive/runtime.link/api/grpc.sizeof_map
//go:noescape
func sizeof_map_noescape(n wireNumber, rvalue reflect.Value, depth int) (int, int, error)

//go:linkname encode_map_noescape github.com/iqhive/runtime.link/api/grpc.encode_map
//go:noescape
func encode_map_noescape(e *Encoder, n wireNumber, rvalue reflect.Value, depth int) error

//go:linkname reflect_new_at_noescape github.com/iqhive/runtime.link/api/grpc.reflect_new_at
//go:noescape
func reflect_new_at_noescape(t reflect.Type, p unsafe.Pointer) reflect.Value

func reflect_new_at(t reflect.Type, p unsafe.Pointer) reflect.Value {
	return reflect.NewAt(t, p)
}

func sizeof_map(n wireNumber, rvalue reflect.Value, depth int) (int, int, error) {
	var size int
	iter := rvalue.MapRange()
	var keyBuf, valBuf [3]uintptr
	var key, val reflect.Value
	if rvalue.Type().Key().Size() < unsafe.Sizeof(keyBuf) {
		key = reflect_new_at_noescape(rvalue.Type().Key(), unsafe.Pointer(&keyBuf)).Elem()
	} else {
		key = reflect.New(rvalue.Type().Key()).Elem()
	}
	if rvalue.Type().Elem().Size() < unsafe.Sizeof(valBuf) {
		val = reflect_new_at_noescape(rvalue.Type().Elem(), unsafe.Pointer(&valBuf)).Elem()
	} else {
		val = reflect.New(rvalue.Type().Elem()).Elem()
	}
	for iter.Next() {
		key.SetIterKey(iter)
		val.SetIterValue(iter)
		keyHead, keyBody, err := sizeof(1, key, depth+1, false)
		if err != nil {
			return 0, 0, err
		}
		valHead, valBody, err := sizeof(2, val, depth+1, false)
		if err != nil {
			return 0, 0, err
		}
		body := keyHead + keyBody + valHead + valBody
		size += sizeTag(n) + sizeVarint(uint64(body)) + body
	}
	return 0, size, nil
}

func encode_map(e *Encoder, n wireNumber, rvalue reflect.Value, depth int) error {
	var stack [16]byte
	var slice = stack[0:0:cap(stack)]
	iter := rvalue.MapRange()
	var keyBuf, valBuf [3]uintptr
	var key, val reflect.Value
	if rvalue.Type().Key().Size() < unsafe.Sizeof(keyBuf) {
		key = reflect_new_at_noescape(rvalue.Type().Key(), unsafe.Pointer(&keyBuf)).Elem()
	} else {
		key = reflect.New(rvalue.Type().Key()).Elem()
	}
	if rvalue.Type().Elem().Size() < unsafe.Sizeof(valBuf) {
		val = reflect_new_at_noescape(rvalue.Type().Elem(), unsafe.Pointer(&valBuf)).Elem()
	} else {
		val = reflect.New(rvalue.Type().Elem()).Elem()
	}
	for iter.Next() {
		key.SetIterKey(iter)
		val.SetIterValue(iter)
		keyHead, keyBody, err := sizeof(1, key, depth+1, false)
		if err != nil {
			return err
		}
		valHead, valBody, err := sizeof(2, val, depth+1, false)
		if err != nil {
			return err
		}
		size := keyHead + keyBody + valHead + valBody
		slice = stack[0:0:cap(stack)]
		slice = appendTag(slice[:], n, typeBytes)
		slice = appendVarint(slice[:], uint64(size))
		if _, err := write(e.w, slice); err != nil {
			return err
		}
		if err := e.encode(1, key, depth+1, false); err != nil {
			return err
		}
		if err := e.encode(2, val, depth+1, false); err != nil {
			return err
		}
	}
	return nil
}
