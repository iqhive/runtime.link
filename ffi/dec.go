package ffi

import (
	"fmt"
	"reflect"
	"strings"
)

type statefulDecoder struct {
	value reflect.Value
	state int
}

func (ffi *API) Decode(dst any, val Structure) {
	rtype := reflect.TypeOf(dst)
	value := reflect.ValueOf(dst)
	if rtype.Kind() != reflect.Ptr {
		panic("ffi.Decode: dst must be a pointer")
	}
	rtype = rtype.Elem()
	value = value.Elem()
	switch rtype.Kind() {
	case reflect.Bool:
		value.SetBool(ffi.Structure.Decode.Bool(val))
	case reflect.Int:
		value.SetInt(int64(ffi.Structure.Decode.Int(val)))
	case reflect.Int8:
		value.SetInt(int64(ffi.Structure.Decode.Int8(val)))
	case reflect.Int16:
		value.SetInt(int64(ffi.Structure.Decode.Int16(val)))
	case reflect.Int32:
		value.SetInt(int64(ffi.Structure.Decode.Int32(val)))
	case reflect.Int64:
		value.SetInt(ffi.Structure.Decode.Int64(val))
	case reflect.Uint:
		value.SetUint(uint64(ffi.Structure.Decode.Uint(val)))
	case reflect.Uint8:
		value.SetUint(uint64(ffi.Structure.Decode.Uint8(val)))
	case reflect.Uint16:
		value.SetUint(uint64(ffi.Structure.Decode.Uint16(val)))
	case reflect.Uint32:
		value.SetUint(uint64(ffi.Structure.Decode.Uint32(val)))
	case reflect.Uint64:
		value.SetUint(ffi.Structure.Decode.Uint64(val))
	case reflect.Uintptr:
		value.SetUint(uint64(ffi.Structure.Decode.Uintptr(val)))
	case reflect.Float32:
		value.SetFloat(float64(ffi.Structure.Decode.Float32(val)))
	case reflect.Float64:
		value.SetFloat(ffi.Structure.Decode.Float64(val))
	case reflect.Complex64:
		value.SetComplex(complex128(ffi.Structure.Decode.Complex64(val)))
	case reflect.Complex128:
		value.SetComplex(ffi.Structure.Decode.Complex128(val))
	case reflect.Array:
		for i := range rtype.Len() {
			ffi.Decode(value.Index(i).Addr().Interface(), val)
		}
	case reflect.Chan:
		value.Set(reflect.ValueOf(ffi.Structure.Decode.Chan(val)))
	case reflect.Func:
		value.Set(reflect.ValueOf(ffi.Structure.Decode.Func(val)))
	case reflect.Interface:
		ffi.Decode(value.Interface(), val)
	case reflect.Map:
		value.Set(reflect.ValueOf(ffi.Structure.Decode.Map(val)))
	case reflect.Ptr:
		ffi.Decode(value.Interface(), val)
	case reflect.Slice:
		value.Set(reflect.ValueOf(ffi.Structure.Decode.Slice(val)))
	case reflect.String:
		str := ffi.Structure.Decode.String(val)
		defer ffi.String.Free(str)
		data := ffi.String.Data(str)
		defer ffi.Structure.Free(data)
		var b strings.Builder
		for range ffi.String.Len(str) {
			b.WriteByte(ffi.Structure.Decode.Uint8(data))
		}
		value.SetString(b.String())
	case reflect.Struct:
		ffi.Decode(value.Addr().Interface(), val)
	case reflect.UnsafePointer:
		value.Set(reflect.ValueOf(ffi.Structure.Decode.Pointer(val)))
	}
}

func decode[T any](structure reflect.Value) T {
	switch structure.Kind() {
	case reflect.Chan:
		val, ok := structure.Recv()
		if !ok {
			panic("ffi.decode: chan closed")
		}
		return val.Interface().(T)
	case reflect.Interface:
		return structure.Interface().(T)
	case reflect.Func:
		panic("ffi.decode: func not implemented")
	case reflect.Map:
		panic("ffi.decode: map not implemented")
	case reflect.Pointer:
		if structure.Type() == reflect.TypeFor[*statefulDecoder]() {
			decoder := structure.Interface().(*statefulDecoder)
			index := decoder.state
			decoder.state++
			if elemType := structure.Type().Elem(); elemType == reflect.TypeFor[reflect.Value]() {
				return decoder.value.Index(index).Interface().(reflect.Value).Interface().(T)
			}
			return decoder.value.Index(index).Interface().(T)
		}
		return structure.Elem().Interface().(T)
	case reflect.Slice:
		panic("ffi.decode: stateless slice not implemented")
	case reflect.String:
		panic("ffi.decode: string not implemented")
	default:
		panic(fmt.Sprintf("ffi.decode: illegal decode %v", structure.Type()))
	}
}

func decodeRef(structure reflect.Value, state int) uint64 {
	switch structure.Kind() {
	case reflect.Chan:
		val, ok := structure.Recv()
		if !ok {
			panic("ffi.decode: chan closed")
		}
		return new_ref(val)
	case reflect.Func:
		panic("ffi.decode: func not implemented")
	case reflect.Map:
		panic("ffi.decode: map not implemented")
	case reflect.Pointer:
		if structure.Type() == reflect.TypeFor[*statefulDecoder]() {
			decoder := structure.Interface().(*statefulDecoder)
			defer func() {
				decoder.state++
			}()
			return decodeRef(decoder.value, decoder.state)
		}
		return new_ref(structure.Elem())
	case reflect.Slice:
		if elemType := structure.Type().Elem(); elemType == reflect.TypeFor[reflect.Value]() {
			return new_ref(structure.Index(state).Interface().(reflect.Value))
		}
		return new_ref(structure.Index(state))
	case reflect.String:
		panic("ffi.decode: string not implemented")
	case reflect.Struct:
		panic("ffi.decode: struct not implemented")
	default:
		panic(fmt.Sprintf("ffi.decodeRef: illegal decode %v", structure.Type()))
	}
}
